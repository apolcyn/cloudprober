// Copyright 2017-2023 The Cloudprober Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package external implements an external probe type for cloudprober.

External probe type executes an external process for actual probing. These probes
can have two modes: "once" and "server". In "once" mode, the external process is
started for each probe run cycle, while in "server" mode, external process is
started only if it's not running already and Cloudprober communicates with it
over stdin/stdout for each probe cycle.
*/
package external

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudprober/cloudprober/common/strtemplate"
	"github.com/cloudprober/cloudprober/logger"
	"github.com/cloudprober/cloudprober/metrics"
	"github.com/cloudprober/cloudprober/metrics/payload"
	configpb "github.com/cloudprober/cloudprober/probes/external/proto"
	serverpb "github.com/cloudprober/cloudprober/probes/external/proto"
	"github.com/cloudprober/cloudprober/probes/external/serverutils"
	"github.com/cloudprober/cloudprober/probes/options"
	"github.com/cloudprober/cloudprober/targets/endpoint"
	"github.com/cloudprober/cloudprober/validators"
	"github.com/google/shlex"
	"google.golang.org/protobuf/proto"
)

var (
	// TimeBetweenRequests is the time interval between probe requests for
	// multiple targets. In server mode, probe requests for multiple targets are
	// sent to the same external probe process. Sleeping between requests provides
	// some time buffer for the probe process to dequeue the incoming requests and
	// avoids filling up the communication pipe.
	//
	// Note that this value impacts the effective timeout for a target as timeout
	// is applied for all the targets in aggregate. For example, 100th target in
	// the targets list will have the effective timeout of (timeout - 1ms).
	// TODO(manugarg): Make sure that the last target in the list has an impact of
	// less than 1% on its timeout.
	TimeBetweenRequests = 10 * time.Microsecond
	validLabelRe        = regexp.MustCompile(`@(target|address|port|probe|target\.label\.[^@]+)@`)
)

type result struct {
	total, success    int64
	latency           metrics.Value
	validationFailure *metrics.Map
}

// Probe holds aggregate information about all probe runs, per-target.
type Probe struct {
	name    string
	mode    string
	cmdName string
	cmdArgs []string
	envVars []string
	opts    *options.Options
	c       *configpb.ProbeConf
	l       *logger.Logger

	// book-keeping params
	labelKeys  map[string]bool // Labels for substitution
	requestID  int32
	cmdRunning bool
	cmdStdin   io.Writer
	cmdStdout  io.ReadCloser
	cmdStderr  io.ReadCloser
	replyChan  chan *serverpb.ProbeReply
	targets    []endpoint.Endpoint
	results    map[string]*result // probe results keyed by targets
	dataChan   chan *metrics.EventMetrics

	// This is used for overriding run command logic for testing.
	runCommandFunc func(ctx context.Context, cmd string, args, envVars []string) ([]byte, []byte, error)

	// default payload metrics that we clone from to build per-target payload
	// metrics.
	payloadParser *payload.Parser
}

func (p *Probe) updateLabelKeys() {
	p.labelKeys = make(map[string]bool)

	updateLabelKeysFn := func(s string) {
		matches := validLabelRe.FindAllStringSubmatch(s, -1)
		for _, m := range matches {
			if len(m) >= 2 {
				// Pick the match within outer parentheses.
				p.labelKeys[m[1]] = true
			}
		}
	}

	for _, opt := range p.c.GetOptions() {
		updateLabelKeysFn(opt.GetValue())
	}
	for _, arg := range p.cmdArgs {
		updateLabelKeysFn(arg)
	}
}

// Init initializes the probe with the given params.
func (p *Probe) Init(name string, opts *options.Options) error {
	c, ok := opts.ProbeConf.(*configpb.ProbeConf)
	if !ok {
		return fmt.Errorf("not external probe config")
	}
	p.name = name
	p.opts = opts
	if p.l = opts.Logger; p.l == nil {
		p.l = &logger.Logger{}
	}
	p.c = c
	p.replyChan = make(chan *serverpb.ProbeReply)

	cmdParts, err := shlex.Split(p.c.GetCommand())
	if err != nil {
		return fmt.Errorf("error parsing command line (%s): %v", p.c.GetCommand(), err)
	}
	p.cmdName = cmdParts[0]
	p.cmdArgs = cmdParts[1:]

	for k, v := range p.c.GetEnvVar() {
		if v == "" {
			v = "1" // default to a truthy value
		}
		p.envVars = append(p.envVars, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(p.envVars)

	// Figure out labels we are interested in
	p.updateLabelKeys()

	switch p.c.GetMode() {
	case configpb.ProbeConf_ONCE:
		p.mode = "once"
	case configpb.ProbeConf_SERVER:
		p.mode = "server"
	default:
		return fmt.Errorf("invalid mode: %s", p.c.GetMode())
	}

	p.results = make(map[string]*result)

	if !p.c.GetOutputAsMetrics() {
		return nil
	}

	defaultKind := metrics.CUMULATIVE
	if p.c.GetMode() == configpb.ProbeConf_ONCE {
		defaultKind = metrics.GAUGE
	}

	p.payloadParser, err = payload.NewParser(p.c.GetOutputMetricsOptions(), "external", p.name, metrics.Kind(defaultKind), p.l)
	if err != nil {
		return fmt.Errorf("error initializing payload metrics: %v", err)
	}

	return nil
}

type command interface {
	Wait() error
}

// monitorCommand waits for the process to terminate and sets cmdRunning to
// false when that happens.
func (p *Probe) monitorCommand(startCtx context.Context, cmd command) error {
	err := cmd.Wait()

	// Spare logging error message if killed explicitly.
	select {
	case <-startCtx.Done():
		return nil
	default:
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("external probe process died with the status: %s. Stderr: %s", exitErr.Error(), string(exitErr.Stderr))
	}
	return err
}

func (p *Probe) startCmdIfNotRunning(startCtx context.Context) error {
	// Start external probe command if it's not running already. Note that here we
	// are trusting the cmdRunning to be set correctly. It can be false for 3 reasons:
	// 1) This is the first call and the process has actually never been started.
	// 2) cmd.Start() started the process but still returned an error.
	// 3) cmd.Wait() returned incorrectly, while the process was still running.
	//
	// 2 or 3 should never happen as per design, but managing processes can be tricky.
	// Documenting here to help with debugging if we run into an issue.
	if p.cmdRunning {
		return nil
	}
	p.l.Infof("Starting external command: %s %s", p.cmdName, strings.Join(p.cmdArgs, " "))
	cmd := exec.CommandContext(startCtx, p.cmdName, p.cmdArgs...)
	var err error
	if p.cmdStdin, err = cmd.StdinPipe(); err != nil {
		return err
	}
	if p.cmdStdout, err = cmd.StdoutPipe(); err != nil {
		return err
	}
	if p.cmdStderr, err = cmd.StderrPipe(); err != nil {
		return err
	}
	if len(p.envVars) > 0 {
		cmd.Env = append(cmd.Env, p.envVars...)
	}

	go func() {
		scanner := bufio.NewScanner(p.cmdStderr)
		for scanner.Scan() {
			p.l.Warningf("Stderr of %s: %s", cmd.Path, scanner.Text())
		}
	}()

	if err = cmd.Start(); err != nil {
		p.l.Errorf("error while starting the cmd: %s %s. Err: %v", cmd.Path, cmd.Args, err)
		return fmt.Errorf("error while starting the cmd: %s %s. Err: %v", cmd.Path, cmd.Args, err)
	}

	doneChan := make(chan struct{})
	// This goroutine waits for the process to terminate and sets cmdRunning to
	// false when that happens.
	go func() {
		if err := p.monitorCommand(startCtx, cmd); err != nil {
			p.l.Error(err.Error())
		}
		close(doneChan)
		p.cmdRunning = false
	}()
	go p.readProbeReplies(doneChan)
	p.cmdRunning = true
	return nil
}

func (p *Probe) readProbeReplies(done chan struct{}) error {
	bufReader := bufio.NewReader(p.cmdStdout)
	// Start a background goroutine to read probe replies from the probe server
	// process's stdout and put them on the probe's replyChan. Note that replyChan
	// is a one element channel. Idea is that we won't need buffering other than
	// the one provided by Unix pipes.
	for {
		select {
		case <-done:
			return nil
		default:
		}
		rep, err := serverutils.ReadProbeReply(bufReader)
		if err != nil {
			// Return if external probe process pipe has closed. We get:
			//  io.EOF: when other process has closed the pipe.
			//  os.ErrClosed: when we have closed the pipe (through cmd.Wait()).
			// *os.PathError: deferred close of the pipe.
			_, isPathError := err.(*os.PathError)
			if err == os.ErrClosed || err == io.EOF || isPathError {
				p.l.Errorf("External probe process pipe is closed. Err: %s", err.Error())
				return err
			}
			p.l.Errorf("Error reading probe reply: %s", err.Error())
			continue
		}
		p.replyChan <- rep
	}

}

func (p *Probe) withAdditionalLabels(em *metrics.EventMetrics, target string) *metrics.EventMetrics {
	for _, al := range p.opts.AdditionalLabels {
		em.AddLabel(al.KeyValueForTarget(endpoint.Endpoint{Name: target}))
	}
	return em
}

func (p *Probe) defaultMetrics(target string, result *result) *metrics.EventMetrics {
	em := metrics.NewEventMetrics(time.Now()).
		AddMetric("success", metrics.NewInt(result.success)).
		AddMetric("total", metrics.NewInt(result.total)).
		AddMetric(p.opts.LatencyMetricName, result.latency).
		AddLabel("ptype", "external").
		AddLabel("probe", p.name).
		AddLabel("dst", target)

	em.LatencyUnit = p.opts.LatencyUnit

	if p.opts.Validators != nil {
		em.AddMetric("validation_failure", result.validationFailure)
	}

	return p.withAdditionalLabels(em, target)
}

func (p *Probe) labels(ep endpoint.Endpoint) map[string]string {
	labels := make(map[string]string)
	if p.labelKeys["probe"] {
		labels["probe"] = p.name
	}
	if p.labelKeys["target"] {
		labels["target"] = ep.Name
	}
	if p.labelKeys["port"] {
		labels["port"] = strconv.Itoa(ep.Port)
	}
	if p.labelKeys["address"] {
		addr, err := p.opts.Targets.Resolve(ep.Name, p.opts.IPVersion)
		if err != nil {
			p.l.Warningf("Targets.Resolve(%v, %v) failed: %v ", ep.Name, p.opts.IPVersion, err)
		} else if !addr.IsUnspecified() {
			labels["address"] = addr.String()
		}
	}
	for lk, lv := range ep.Labels {
		k := "target.label." + lk
		if p.labelKeys[k] {
			labels[k] = lv
		}
	}
	return labels
}

func (p *Probe) sendRequest(requestID int32, ep endpoint.Endpoint) error {
	req := &serverpb.ProbeRequest{
		RequestId: proto.Int32(requestID),
		TimeLimit: proto.Int32(int32(p.opts.Timeout / time.Millisecond)),
		Options:   []*serverpb.ProbeRequest_Option{},
	}
	for _, opt := range p.c.GetOptions() {
		value := opt.GetValue()
		if len(p.labelKeys) != 0 { // If we're looking for substitions.
			res, found := strtemplate.SubstituteLabels(value, p.labels(ep))
			if !found {
				p.l.Warningf("Missing substitution in option %q", value)
			} else {
				value = res
			}
		}
		req.Options = append(req.Options, &serverpb.ProbeRequest_Option{
			Name:  opt.Name,
			Value: proto.String(value),
		})
	}

	p.l.Debugf("Sending a probe request %v to the external probe server for target %v", requestID, ep.Name)
	return serverutils.WriteMessage(req, p.cmdStdin)
}

type requestInfo struct {
	target    string
	timestamp time.Time
}

// probeStatus captures the single probe status. It's only used by runProbe
// functions to pass a probe's status to processProbeResult method.
type probeStatus struct {
	target  string
	success bool
	latency time.Duration
	payload string
}

func (p *Probe) processProbeResult(ps *probeStatus, result *result) {
	if ps.success && p.opts.Validators != nil {
		failedValidations := validators.RunValidators(p.opts.Validators, &validators.Input{ResponseBody: []byte(ps.payload)}, result.validationFailure, p.l)

		// If any validation failed, log and set success to false.
		if len(failedValidations) > 0 {
			p.l.Debug("Target:", ps.target, " failed validations: ", strings.Join(failedValidations, ","), ".")
			ps.success = false
		}
	}

	if ps.success {
		result.success++
		result.latency.AddFloat64(ps.latency.Seconds() / p.opts.LatencyUnit.Seconds())
	}

	em := p.defaultMetrics(ps.target, result)
	p.opts.LogMetrics(em)
	p.dataChan <- em

	// If probe is configured to use the external process output (or reply payload
	// in case of server probe) as metrics.
	if p.c.GetOutputAsMetrics() {
		for _, em := range p.payloadParser.PayloadMetrics(ps.payload, ps.target) {
			p.opts.LogMetrics(em)
			p.dataChan <- p.withAdditionalLabels(em, ps.target)
		}
	}
}

func (p *Probe) runServerProbe(ctx, startCtx context.Context) {
	requests := make(map[int32]requestInfo)
	var requestsMu sync.RWMutex
	doneChan := make(chan struct{})

	if err := p.startCmdIfNotRunning(startCtx); err != nil {
		p.l.Error(err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Read probe replies until we have no outstanding requests or context has
		// run out.
		for {
			_, ok := <-doneChan
			if !ok {
				// It is safe to access requests without lock here as it won't be accessed
				// by the send loop after doneChan is closed.
				p.l.Debugf("Number of outstanding requests: %d", len(requests))
				if len(requests) == 0 {
					return
				}
			}
			select {
			case <-ctx.Done():
				p.l.Error(ctx.Err().Error())
				return
			case rep := <-p.replyChan:
				requestsMu.Lock()
				reqInfo, ok := requests[rep.GetRequestId()]
				if ok {
					delete(requests, rep.GetRequestId())
				}
				requestsMu.Unlock()
				if !ok {
					// Not our reply, could be from the last timed out probe.
					p.l.Warningf("Got a reply that doesn't match any outstading request: Request id from reply: %v. Ignoring.", rep.GetRequestId())
					continue
				}
				success := true
				if rep.GetErrorMessage() != "" {
					p.l.Errorf("Probe for target %v failed with error message: %s", reqInfo.target, rep.GetErrorMessage())
					success = false
				}
				p.processProbeResult(&probeStatus{
					target:  reqInfo.target,
					success: success,
					latency: time.Since(reqInfo.timestamp),
					payload: rep.GetPayload(),
				}, p.results[reqInfo.target])
			}
		}
	}()

	// Send probe requests
	for _, target := range p.targets {
		p.requestID++
		p.results[target.Name].total++
		requestsMu.Lock()
		requests[p.requestID] = requestInfo{
			target:    target.Name,
			timestamp: time.Now(),
		}
		requestsMu.Unlock()
		p.sendRequest(p.requestID, target)
		time.Sleep(TimeBetweenRequests)
	}

	// Send signal to receiver loop that we are done sending request.
	close(doneChan)

	// Wait for receiver goroutine to exit.
	wg.Wait()

	// Handle requests that we have not yet received replies for: "requests" will
	// contain only outstanding requests by this point.
	requestsMu.Lock()
	defer requestsMu.Unlock()
	for _, req := range requests {
		p.processProbeResult(&probeStatus{
			target:  req.target,
			success: false,
		}, p.results[req.target])
	}
}

func (p *Probe) runOnceProbe(ctx context.Context) {
	var wg sync.WaitGroup

	for _, target := range p.targets {
		wg.Add(1)
		go func(target endpoint.Endpoint, result *result) {
			defer wg.Done()

			args := append([]string{}, p.cmdArgs...)
			if len(p.labelKeys) != 0 {
				for i, arg := range p.cmdArgs {
					res, found := strtemplate.SubstituteLabels(arg, p.labels(target))
					if !found {
						p.l.Warningf("Substitution not found in %q", arg)
					}
					args[i] = res
				}
			}

			p.l.Infof("Running external command: %s %s", p.cmdName, strings.Join(args, " "))
			result.total++
			startTime := time.Now()

			var stdout, stderr []byte
			var err error
			if p.runCommandFunc != nil {
				stdout, stderr, err = p.runCommandFunc(ctx, p.cmdName, args, p.envVars)
			} else {
				stdout, stderr, err = p.runCommand(ctx, p.cmdName, args, p.envVars)
			}

			success := true
			if err != nil {
				success = false
				if exitErr, ok := err.(*exec.ExitError); ok {
					p.l.Errorf("external probe process died with the status: %s. Stderr: %s", exitErr.Error(), stderr)
				} else {
					p.l.Errorf("Error executing the external program. Err: %v", err)
				}
			} else {
				if len(stderr) != 0 {
					p.l.Warningf("Stderr: %s", stderr)
				}
			}

			p.processProbeResult(&probeStatus{
				target:  target.Name,
				success: success,
				latency: time.Since(startTime),
				payload: string(stdout),
			}, result)
		}(target, p.results[target.Name])
	}
	wg.Wait()
}

func (p *Probe) updateTargets() {
	p.targets = p.opts.Targets.ListEndpoints()

	for _, target := range p.targets {
		if _, ok := p.results[target.Name]; ok {
			continue
		}

		var latencyValue metrics.Value
		if p.opts.LatencyDist != nil {
			latencyValue = p.opts.LatencyDist.Clone()
		} else {
			latencyValue = metrics.NewFloat(0)
		}

		p.results[target.Name] = &result{
			latency:           latencyValue,
			validationFailure: validators.ValidationFailureMap(p.opts.Validators),
		}

		for _, al := range p.opts.AdditionalLabels {
			// Note it's a bit convoluted right now because we want to use the
			// same key while updating additional labels that we use while
			// retrieving additional labels in withAdditionalLabels.
			al.UpdateForTarget(endpoint.Endpoint{Name: target.Name}, "", 0)
		}
	}
}

func (p *Probe) runProbe(startCtx context.Context) {
	probeCtx, cancelFunc := context.WithTimeout(startCtx, p.opts.Timeout)
	defer cancelFunc()

	p.updateTargets()

	if p.mode == "server" {
		p.runServerProbe(probeCtx, startCtx)
	} else {
		p.runOnceProbe(probeCtx)
	}
}

// Start starts and runs the probe indefinitely.
func (p *Probe) Start(startCtx context.Context, dataChan chan *metrics.EventMetrics) {
	p.dataChan = dataChan

	ticker := time.NewTicker(p.opts.Interval)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		select {
		case <-startCtx.Done():
			return
		default:
		}

		p.runProbe(startCtx)
	}
}
