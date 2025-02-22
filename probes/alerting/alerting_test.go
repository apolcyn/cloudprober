// Copyright 2023 The Cloudprober Authors.
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

package alerting

import (
	"strconv"
	"testing"
	"time"

	"github.com/cloudprober/cloudprober/metrics"
	configpb "github.com/cloudprober/cloudprober/probes/alerting/proto"
	"github.com/cloudprober/cloudprober/targets/endpoint"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func testAlertInfo(target string, failures, total, dur int) *AlertInfo {
	return &AlertInfo{
		Name:         "test-probe",
		ProbeName:    "test-probe",
		Target:       endpoint.Endpoint{Name: target},
		Failures:     failures,
		Total:        total,
		FailingSince: time.Time{}.Add(time.Duration(dur) * time.Second),
		ConditionID:  strconv.FormatInt(time.Time{}.Add(time.Duration(dur)*time.Second).Unix(), 10),
	}
}

type testData struct {
	total, success []int64
}

type testAlertHandlerArgs struct {
	name        string
	condition   *configpb.Condition
	targets     map[string]testData
	wantAlerted map[string]bool
	wantAlerts  []*AlertInfo
	wantErr     bool
	notifyCfg   *configpb.NotifyConfig
	waitTime    time.Duration
}

func testAlertHandlerBehavior(t *testing.T, tt testAlertHandlerArgs) {
	ah := NewAlertHandler(&configpb.AlertConf{
		Condition: tt.condition,
		Notify:    tt.notifyCfg,
	}, "test-probe", nil)
	ah.notifyCh = make(chan *AlertInfo, 10)

	for target, td := range tt.targets {
		ep := endpoint.Endpoint{Name: target}
		ts := time.Time{}

		for i := range td.total {
			em := metrics.NewEventMetrics(ts)
			em.AddMetric("total", metrics.NewInt(td.total[i]))
			em.AddMetric("success", metrics.NewInt(td.success[i]))

			if err := ah.Record(ep, em); (err != nil) != tt.wantErr {
				t.Errorf("AlertHandler.Record() error = %v, wantErr %v", err, tt.wantErr)
			}
			t.Logf("target (%s) state: %+v", target, ah.targets[ep.Key()])

			ts = ts.Add(time.Second)
			time.Sleep(tt.waitTime)
		}

		// Verify that target is in expected alerted state after the
		// run.
		assert.Equal(t, tt.wantAlerted[target], ah.targets[ep.Key()].alerted, target+" alerted")
	}

	// Verify that alerts are sent on the notify channel.
	assert.Equal(t, len(tt.wantAlerts), len(ah.notifyCh), "number of alerts")
	if len(tt.wantAlerts) == len(ah.notifyCh) {
		for i := range tt.wantAlerts {
			a := <-ah.notifyCh
			assert.Equal(t, tt.wantAlerts[i], a)
		}
	}
}

func TestAlertHandlerRecord(t *testing.T) {
	tests := []struct {
		name           string
		condition      *configpb.Condition
		total, success []int64
		wantAlerted    bool
		wantAlerts     []*AlertInfo
	}{
		{
			name:        "single-target-no-alert",
			total:       []int64{1, 2},
			success:     []int64{1, 2},
			wantAlerted: false,
		},
		{
			name:        "single-target-alert-default-condition",
			total:       []int64{1, 2, 3},
			success:     []int64{1, 2, 2}, // Success didn't increase.
			wantAlerted: true,
			wantAlerts:  []*AlertInfo{testAlertInfo("target1", 1, 1, 2)},
		},
		{
			name:        "default-condition-one-point-no-alert",
			total:       []int64{2},
			success:     []int64{1},
			wantAlerted: false,
		},
		{
			name:        "alerts-last-alert-cleared",
			total:       []int64{2, 4, 6, 8},
			success:     []int64{1, 3, 4, 6},
			wantAlerted: false,
			wantAlerts:  []*AlertInfo{testAlertInfo("target1", 1, 1, 2)},
		},
		{
			name:        "alert-over-a-period-of-time",
			condition:   &configpb.Condition{Failures: int32(3), Total: int32(5)},
			total:       []int64{2, 4, 6, 8}, // total: 2, 2, 2
			success:     []int64{1, 2, 4, 4}, // failures: 1, 0, 2
			wantAlerted: true,
			wantAlerts:  []*AlertInfo{testAlertInfo("target1", 3, 5, 3)},
		},
		{
			name:        "over-a-period-of-time-alert-cleared",
			condition:   &configpb.Condition{Failures: int32(3), Total: int32(5)},
			total:       []int64{2, 4, 6, 8, 10}, // total: 2, 2, 2, 2
			success:     []int64{1, 2, 4, 4, 6},  // failures: 1, 0, 2, 0
			wantAlerted: false,
			wantAlerts:  []*AlertInfo{testAlertInfo("target1", 3, 5, 3)},
		},
		{
			name:      "alert-cleared-and-alerted-again",
			condition: &configpb.Condition{Failures: int32(3), Total: int32(5)},
			total:     []int64{2, 4, 6, 8, 10, 12}, // total:    2, 2, 2, 2, 2
			success:   []int64{1, 2, 4, 4, 6, 6},   // failures: 1, 0, 2, 0, 2
			// total:    2, 2, 2, 2, 2
			// failures: 1, 0, 2, 0, 2
			wantAlerted: true,
			wantAlerts:  []*AlertInfo{testAlertInfo("target1", 3, 5, 3), testAlertInfo("target1", 3, 5, 5)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAlertHandlerBehavior(t, testAlertHandlerArgs{
				condition:   tt.condition,
				targets:     map[string]testData{"target1": {total: tt.total, success: tt.success}},
				wantAlerted: map[string]bool{"target1": tt.wantAlerted},
				wantAlerts:  tt.wantAlerts,
			})
		})
	}
}

func TestNotificationRepeat(t *testing.T) {
	tests := []struct {
		name           string
		condition      *configpb.Condition
		notifyCfg      *configpb.NotifyConfig
		total, success []int64
		waitTime       time.Duration
		wantAlerts     []*AlertInfo
	}{
		{
			name:       "continuous-condition-single-notification",
			total:      []int64{1, 2, 3},
			success:    []int64{1, 1, 1},
			waitTime:   10 * time.Millisecond,
			wantAlerts: []*AlertInfo{testAlertInfo("target1", 1, 1, 1)},
		},
		{
			name:       "continuous-condition-repeat-notification",
			notifyCfg:  &configpb.NotifyConfig{RepeatIntervalSec: proto.Int32(0)},
			total:      []int64{1, 2, 3},
			success:    []int64{1, 1, 1},
			waitTime:   10 * time.Millisecond,
			wantAlerts: []*AlertInfo{testAlertInfo("target1", 1, 1, 1), testAlertInfo("target1", 1, 1, 1)},
		},
		{
			name:       "continuous-condition-no-repeat-yet",
			notifyCfg:  &configpb.NotifyConfig{RepeatIntervalSec: proto.Int32(1)},
			total:      []int64{1, 2, 3, 4},
			success:    []int64{1, 1, 1, 1},
			waitTime:   10 * time.Millisecond,
			wantAlerts: []*AlertInfo{testAlertInfo("target1", 1, 1, 1)},
		},
		{
			name:       "continuous-condition-for-1-sec",
			notifyCfg:  &configpb.NotifyConfig{RepeatIntervalSec: proto.Int32(1)},
			total:      []int64{1, 2, 3, 4, 5, 6, 7, 8},
			success:    []int64{1, 1, 1, 1, 1, 1, 1, 1},
			waitTime:   200 * time.Millisecond,
			wantAlerts: []*AlertInfo{testAlertInfo("target1", 1, 1, 1), testAlertInfo("target1", 1, 1, 1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAlertHandlerBehavior(t, testAlertHandlerArgs{
				condition:   tt.condition,
				notifyCfg:   tt.notifyCfg,
				targets:     map[string]testData{"target1": {total: tt.total, success: tt.success}},
				wantAlerted: map[string]bool{"target1": true},
				wantAlerts:  tt.wantAlerts,
				waitTime:    tt.waitTime,
			})
		})
	}
}

func TestAlertHandlerRecordTwoTargets(t *testing.T) {
	tests := []testAlertHandlerArgs{
		{
			name:      "only-target2-alert",
			condition: &configpb.Condition{Failures: int32(2), Total: int32(0)},
			targets: map[string]testData{
				"target1": {
					total:   []int64{1, 2, 3, 4}, // total: 1, 1, 1
					success: []int64{1, 2, 2, 3}, // failures: 0, 1, 0
				},
				"target2": {
					total:   []int64{1, 2, 3, 4}, // total: 1, 1, 1
					success: []int64{1, 2, 2, 2}, // failures: 0, 1, 1
				},
			},
			wantAlerted: map[string]bool{"target1": false, "target2": true},
			wantAlerts:  []*AlertInfo{testAlertInfo("target2", 2, 2, 3)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAlertHandlerBehavior(t, tt)
		})
	}
}

func TestNewAlertHandler(t *testing.T) {
	tests := []struct {
		name      string
		conf      *configpb.AlertConf
		probeName string
		want      *AlertHandler
	}{
		{
			name:      "default-condition",
			probeName: "test-probe",
			conf: &configpb.AlertConf{
				Name: "test-alert",
			},
			want: &AlertHandler{
				name:         "test-alert",
				probeName:    "test-probe",
				condition:    &configpb.Condition{Failures: 1, Total: 1},
				targets:      make(map[string]*targetState),
				notifyConfig: &configpb.NotifyConfig{RepeatIntervalSec: proto.Int32(3600)},
			},
		},
		{
			name:      "no-alert-name",
			probeName: "test-probe",
			conf: &configpb.AlertConf{
				Condition: &configpb.Condition{Failures: 4, Total: 5},
			},
			want: &AlertHandler{
				name:         "test-probe",
				probeName:    "test-probe",
				condition:    &configpb.Condition{Failures: 4, Total: 5},
				targets:      make(map[string]*targetState),
				notifyConfig: &configpb.NotifyConfig{RepeatIntervalSec: proto.Int32(3600)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NewAlertHandler(tt.conf, tt.probeName, nil))
		})
	}
}
