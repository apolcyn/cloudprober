// Copyright 2017-2021 The Cloudprober Authors.
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
Package stackdriver implements the Stackdriver version of the Surfacer
object. This package allows users to create an initialized Stack Driver
Surfacer and use it to write custom metrics data.
*/
package stackdriver

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/cloudprober/cloudprober/logger"
	"golang.org/x/oauth2/google"
	monitoring "google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"

	"github.com/cloudprober/cloudprober/metrics"
	"github.com/cloudprober/cloudprober/surfacers/common/options"
	configpb "github.com/cloudprober/cloudprober/surfacers/stackdriver/proto"
)

const batchSize = 200

//-----------------------------------------------------------------------------
// Stack Driver Surfacer Specific Code
//-----------------------------------------------------------------------------

// SDSurfacer structure for StackDriver, which includes an authenticated client
// for making StackDriver API calls, and a registered which is in charge of
// keeping track of what metrics have already been registereded
type SDSurfacer struct {
	c    *configpb.SurfacerConf
	opts *options.Options

	// Metrics regexp
	allowedMetricsRegex *regexp.Regexp

	// Internal cache for saving metric data until a batch is sent
	cache        map[string]*monitoring.TimeSeries
	knownMetrics map[string]bool

	// Channel for writing the data without blocking
	writeChan chan *metrics.EventMetrics

	// VM Information
	onGCE       bool
	projectName string
	resource    *monitoring.MonitoredResource

	// Time when stackdriver module was initialized. This is used as start time
	// for cumulative metrics.
	startTime time.Time

	// Cloud logger
	l       *logger.Logger
	failCnt int64

	// Monitoring client
	client *monitoring.Service
}

// New initializes a SDSurfacer for Stack Driver with all its necessary internal
// variables for call references (project and instances variables) as well
// as provisioning it with clients for making the necessary API calls. New
// requires you to pass in a valid stackdriver surfacer configuration.
func New(ctx context.Context, config *configpb.SurfacerConf, opts *options.Options, l *logger.Logger) (*SDSurfacer, error) {
	// Create a cache, which is used for batching write requests together,
	// and a channel for writing data.
	s := SDSurfacer{
		cache:        make(map[string]*monitoring.TimeSeries),
		knownMetrics: make(map[string]bool),
		writeChan:    make(chan *metrics.EventMetrics, config.GetMetricsBufferSize()),
		c:            config,
		opts:         opts,
		projectName:  config.GetProject(),
		startTime:    time.Now(),
		l:            l,
	}

	if s.c.GetAllowedMetricsRegex() != "" {
		l.Warning("allowed_metrics_regex is now deprecated. Please use the common surfacer options: allow_metrics, ignore_metrics.")
		r, err := regexp.Compile(s.c.GetAllowedMetricsRegex())
		if err != nil {
			return nil, err
		}
		s.allowedMetricsRegex = r
	}

	// Find all the necessary information for writing metrics to Stack
	// Driver.
	var err error

	if metadata.OnGCE() {
		s.onGCE = true

		if s.projectName == "" {
			if s.projectName, err = metadata.ProjectID(); err != nil {
				return nil, fmt.Errorf("unable to retrieve project name: %v", err)
			}
		}

		mr, err := monitoredResourceOnGCE(s.projectName, l)
		if err != nil {
			return nil, fmt.Errorf("error initializing monitored resource for stackdriver on GCE: %v", err)
		}

		s.resource = mr

	}

	// Create monitoring client
	httpClient, err := google.DefaultClient(ctx, monitoring.CloudPlatformScope)
	if err != nil {
		return nil, err
	}
	s.client, err = monitoring.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	// Start either the writeAsync or the writeBatch, depending on if we are
	// batching or not.
	go s.writeBatch(ctx)

	s.l.Info("Created a new stackdriver surfacer")
	return &s, nil
}

// Write queues a message to be written to stackdriver.
func (s *SDSurfacer) Write(_ context.Context, em *metrics.EventMetrics) {
	// Write inserts the data to be written into channel. This channel is
	// watched by writeBatch and will make the necessary calls to the Stackdriver
	// API to write the data from the channel.
	select {
	case s.writeChan <- em:
	default:
		s.l.Errorf("SDSurfacer's write channel is full, dropping new data.")
	}
}

// createMetricDescriptor creates metric descriptor for the given timeseries.
// We create metric descriptors explicitly, instead of relying on auto-
// creation by creating timeseries, because auto-creation doesn't add units to
// the metric.
func (s *SDSurfacer) createMetricDescriptor(ts *monitoring.TimeSeries) error {
	var labels []*monitoring.LabelDescriptor
	for k := range ts.Metric.Labels {
		labels = append(labels, &monitoring.LabelDescriptor{
			Key:       k,
			ValueType: "STRING",
		})
	}

	_, err := s.client.Projects.MetricDescriptors.Create("projects/"+s.projectName, &monitoring.MetricDescriptor{
		Name:       "projects/" + s.projectName + "/metricDescriptors/" + ts.Metric.Type,
		Type:       ts.Metric.Type,
		MetricKind: ts.MetricKind,
		Labels:     labels,
		Unit:       ts.Unit,
		ValueType:  ts.ValueType,
	}).Do()

	return err
}

// writeBatch polls the writeChan and the sendChan waiting for either a new
// write packet or a new context. If data comes in on the writeChan, then
// the data is pulled off and put into the cache (if there is already an
// entry into the cache for the same metric, it updates the metric to the
// new data). If ticker fires, then the metrics in the cache
// are batched together. The Stackdriver API has a limit on the maximum number
// of metrics that can be sent in a single request, so we may have to make
// multiple requests to the Stackdriver API to send the full cache of metrics.
//
// writeBatch is set up to run as an infinite goroutine call in the New function
// to allow it to write asynchronously to Stack Driver.
func (s *SDSurfacer) writeBatch(ctx context.Context) {
	// Introduce a random delay before starting the loop.
	rand.Seed(time.Now().UnixNano())
	randomDelay := time.Duration(rand.Int63n(int64(s.c.GetBatchTimerSec()))) * time.Second
	time.Sleep(randomDelay)

	batchTicker := time.NewTicker(time.Duration(s.c.GetBatchTimerSec()) * time.Second)
	for {
		select {
		case <-ctx.Done():
			s.l.Infof("Context canceled, stopping the input processing loop.")
			batchTicker.Stop()
			return
		case em := <-s.writeChan:
			// Process EventMetrics to build timeseries using them and cache the timeseries
			// objects.
			s.recordEventMetrics(em)
		case <-batchTicker.C:
			// Empty time series writes cause an error to be returned, so
			// we skip any calls that write but wouldn't set any data.
			if len(s.cache) == 0 {
				break
			}

			var ts []*monitoring.TimeSeries
			for _, v := range s.cache {
				if !s.knownMetrics[v.Metric.Type] && v.Unit != "" {
					if err := s.createMetricDescriptor(v); err != nil {
						s.l.Warningf("Error creating metric descriptor for: %s, err: %v", v.Metric.Type, err)
						continue
					}
					s.knownMetrics[v.Metric.Type] = true
				}
				ts = append(ts, v)
			}

			// We batch the time series into appropriately-sized sets
			// and write them
			for i := 0; i < len(ts); i += batchSize {
				endIndex := min(len(ts), i+batchSize)

				s.l.Infof("Sending entries %d through %d of %d", i, endIndex, len(ts))

				// Now that we've created the new metric, we can write the data. Making
				// a time series create call will automatically register a new metric
				// with the correct information if it does not already exist.
				// Ref: https://cloud.google.com/monitoring/custom-metrics/creating-metrics#auto-creation
				requestBody := monitoring.CreateTimeSeriesRequest{
					TimeSeries: ts[i:endIndex],
				}
				if _, err := s.client.Projects.TimeSeries.Create("projects/"+s.projectName, &requestBody).Do(); err != nil {
					s.failCnt++
					s.l.Warningf("Unable to fulfill TimeSeries Create call. Err: %v", err)
				}
			}

			// Flush the cache after we've finished writing so we don't accidentally
			// re-write metric values that haven't been written over several write
			// cycles.
			for k := range s.cache {
				delete(s.cache, k)
			}
		}
	}

}

//-----------------------------------------------------------------------------
// StackDriver Object Creation and Helper Functions
//-----------------------------------------------------------------------------

// recordTimeSeries forms a timeseries object from the given arguments, records
// it in the cache if batch processing is enabled, and returns it.
//
// More information on the object and specific fields can be found here:
//	https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeSeries
func (s *SDSurfacer) recordTimeSeries(metricKind, metricName, msgType string, labels map[string]string, timestamp time.Time, tv *monitoring.TypedValue, unit, cacheKey string) *monitoring.TimeSeries {
	startTime := s.startTime.Format(time.RFC3339Nano)
	if metricKind == "GAUGE" {
		startTime = timestamp.Format(time.RFC3339Nano)
	}

	ts := &monitoring.TimeSeries{
		// The URL address for our custom metric, must match the
		// name we used in the MetricDescriptor.
		Metric: &monitoring.Metric{
			Type:   s.c.GetMonitoringUrl() + metricName,
			Labels: labels,
		},

		// Must match the MetricKind and ValueType of the MetricDescriptor.
		MetricKind: metricKind,
		ValueType:  msgType,
		Unit:       unit,

		// Create a single data point, this could be utilized to create
		// a batch of points instead of a single point if the write
		// rate is too high.
		Points: []*monitoring.Point{
			{
				Interval: &monitoring.TimeInterval{
					StartTime: startTime,
					EndTime:   timestamp.Format(time.RFC3339Nano),
				},
				Value: tv,
			},
		},
	}

	if s.resource != nil {
		ts.Resource = s.resource
	}

	// We create a key that is a composite of both the name and the
	// labels so we can make sure that the cache holds all distinct
	// values and not just the ones with different names.
	s.cache[metricName+","+cacheKey] = ts

	return ts

}

// sdKind converts EventMetrics kind to StackDriver kind string.
func (s *SDSurfacer) sdKind(kind metrics.Kind) string {
	switch kind {
	case metrics.GAUGE:
		return "GAUGE"
	case metrics.CUMULATIVE:
		return "CUMULATIVE"
	default:
		return ""
	}
}

// processLabels processes EventMetrics labels to generate:
//	- a map of label key values to use in StackDriver timeseries,
//	- a labels key of the form label1_key=label1_val,label2_key=label2_val,
//	  used for caching.
//	- prefix for metric names, usually <ptype>/<probe>.
func (s *SDSurfacer) processLabels(em *metrics.EventMetrics) (labels map[string]string, labelsKey, metricPrefix string) {
	labels = make(map[string]string)
	var sortedLabels []string // we use this for cache key below
	var ptype, probe string
	metricPrefixConfig := s.c.GetMetricsPrefix()
	usePType := true && metricPrefixConfig == configpb.SurfacerConf_PTYPE_PROBE
	useProbe := true && metricPrefixConfig == configpb.SurfacerConf_PTYPE_PROBE ||
									metricPrefixConfig == configpb.SurfacerConf_PROBE
	for _, k := range em.LabelsKeys() {
		if k == "ptype" && usePType {
			ptype = em.Label(k)
			continue
		}
		if k == "probe" && useProbe {
			probe = em.Label(k)
			continue
		}
		labels[k] = em.Label(k)
		sortedLabels = append(sortedLabels, k+"="+labels[k])
	}
	labelsKey = strings.Join(sortedLabels, ",")

	if ptype != "" && usePType {
		metricPrefix += ptype + "/"
	}
	if probe != "" && useProbe {
		metricPrefix += probe + "/"
	}
	return
}

func (s *SDSurfacer) ignoreMetric(name string) bool {
	if s.allowedMetricsRegex != nil {
		if !s.allowedMetricsRegex.MatchString(name) {
			return true
		}
	}

	if !validMetricLength(name, s.c.GetMonitoringUrl()) {
		s.l.Warningf("Message name %q is greater than the 100 character limit, skipping write", name)
		return true
	}

	return false
}

// recordEventMetrics processes the incoming EventMetrics objects and builds
// TimeSeries from it.
//
// Since stackdriver doesn't support metrics.String and metrics.Map value types,
// it converts them to a numerical types (stackdriver type Double) with
// additional labels. See the inline comments for this conversion is done.
func (s *SDSurfacer) recordEventMetrics(em *metrics.EventMetrics) (ts []*monitoring.TimeSeries) {
	metricKind := s.sdKind(em.Kind)
	if metricKind == "" {
		s.l.Warningf("Unknown event metrics type (not CUMULATIVE or GAUGE): %v", em.Kind)
		return
	}

	emLabels, cacheKey, metricPrefix := s.processLabels(em)

	for _, k := range em.MetricsKeys() {
		if !s.opts.AllowMetric(k) {
			continue
		}

		// Create a copy of emLabels for use in timeseries object.
		mLabels := make(map[string]string)
		for k, v := range emLabels {
			mLabels[k] = v
		}
		name := metricPrefix + k

		if s.ignoreMetric(name) {
			continue
		}

		// Create the correct TimeSeries object based on the incoming data
		val := em.Metric(k)

		unit := "1" // "1" is the default unit for numbers.
		if k == "latency" {
			unit = map[time.Duration]string{
				time.Second:      "s",
				time.Millisecond: "ms",
				time.Microsecond: "us",
				time.Nanosecond:  "ns",
			}[em.LatencyUnit]
		}

		// If metric value is of type numerical value.
		if v, ok := val.(metrics.NumValue); ok {
			f := float64(v.Int64())
			ts = append(ts, s.recordTimeSeries(metricKind, name, "DOUBLE", mLabels, em.Timestamp, &monitoring.TypedValue{DoubleValue: &f}, unit, cacheKey))
			continue
		}

		// If metric value is of type String.
		if v, ok := val.(metrics.String); ok {
			// Since StackDriver doesn't support string value type for custom metrics,
			// we convert string metrics into a numeric metric with an additional label
			// val="string-val".
			//
			// metrics.String stringer wraps string values in a single "". Remove those
			// for stackdriver.
			mLabels["val"] = strings.Trim(v.String(), "\"")
			f := float64(1)
			ts = append(ts, s.recordTimeSeries(metricKind, name, "DOUBLE", mLabels, em.Timestamp, &monitoring.TypedValue{DoubleValue: &f}, unit, cacheKey))
			continue
		}

		// If metric value is of type Map.
		if mapValue, ok := val.(*metrics.Map); ok {
			// Since StackDriver doesn't support Map value type, we convert Map values
			// to multiple timeseries with map's KeyName and key as labels.
			for _, mapKey := range mapValue.Keys() {
				mmLabels := make(map[string]string)
				for lk, lv := range mLabels {
					mmLabels[lk] = lv
				}
				mmLabels[mapValue.MapName] = mapKey
				f := float64(mapValue.GetKey(mapKey).Int64())
				ts = append(ts, s.recordTimeSeries(metricKind, name, "DOUBLE", mmLabels, em.Timestamp, &monitoring.TypedValue{DoubleValue: &f}, unit, cacheKey))
			}
			continue
		}

		// If metric value is of type Distribution.
		if distValue, ok := val.(*metrics.Distribution); ok {
			ts = append(ts, s.recordTimeSeries(metricKind, name, "DISTRIBUTION", mLabels, em.Timestamp, distValue.StackdriverTypedValue(), unit, cacheKey))
			continue
		}

		// We'll reach here only if encounter an unsupported value type.
		s.l.Warningf("Unsupported value type: %v", val)
	}
	return ts
}

//-----------------------------------------------------------------------------
// Non-stackdriver Helper Functions
//-----------------------------------------------------------------------------

// checkMetricLength checks if the combination of the metricName and the url
// prefix are longer than 100 characters, which is illegal in a Stackdriver
// call. Stack Driver doesn't allow custom metrics with more than 100 character
// names, so we have a check to see if we are going over the limit.
//	Ref: https://cloud.google.com/monitoring/api/v3/metrics#metric_names
func validMetricLength(metricName string, monitoringURL string) bool {
	return len(metricName)+len(monitoringURL) <= 100
}

// Function to return the min of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
