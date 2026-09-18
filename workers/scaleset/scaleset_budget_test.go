// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and limitations
//    under the License.

package scaleset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	dbMocks "github.com/cloudbase/garm/database/common/mocks"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
)

func TestHandleScaleUpStopsAtAuthoritativeOpenCreateCircuit(t *testing.T) {
	store := dbMocks.NewStore(t)
	ctx := context.Background()

	local := params.ScaleSet{
		ID:                 1,
		Enabled:            true,
		MaxRunners:         1,
		DesiredRunnerCount: 1,
		MaxCreateAttempts:  1,
		CreateFailures:     0,
	}
	authoritative := local
	authoritative.CreateFailures = 1

	store.On("GetScaleSetByID", mock.Anything, uint(1)).
		Return(authoritative, nil).
		Once()

	worker := &Worker{
		ctx:      ctx,
		store:    store,
		scaleSet: local,
		runners:  make(map[string]params.Instance),
	}

	// Any attempt to continue past the circuit check would call ControllerInfo
	// on the strict mock and fail the test. This specifically protects against
	// a stale watcher cache allowing one more sequential materialization.
	worker.handleScaleUp()

	if !worker.scaleSet.CreateCircuitOpen() {
		t.Fatal("worker did not retain authoritative open-circuit state")
	}
}


func TestMaterializationFailedBeforeActive(t *testing.T) {
	for _, status := range []params.RunnerStatus{params.RunnerPending, params.RunnerInstalling, params.RunnerFailed} {
		if !materializationFailedBeforeActive(params.Instance{RunnerStatus: status}) {
			t.Fatalf("expected status %q to count as pre-active materialization failure", status)
		}
	}
	for _, status := range []params.RunnerStatus{params.RunnerActive, params.RunnerIdle, params.RunnerTerminated} {
		if materializationFailedBeforeActive(params.Instance{RunnerStatus: status}) {
			t.Fatalf("did not expect status %q to count as pre-active materialization failure", status)
		}
	}
}


func TestHandleJobsStartedResetsMaterializationFailures(t *testing.T) {
	store := dbMocks.NewStore(t)
	ctx := context.Background()
	locker, err := locking.NewLocalLocker(ctx, store)
	if err != nil {
		t.Fatalf("creating local locker: %v", err)
	}
	if err := locking.RegisterLocker(locker); err != nil {
		t.Fatalf("registering local locker: %v", err)
	}

	scaleSet := params.ScaleSet{
		ID:     1,
		RepoID: "11111111-1111-1111-1111-111111111111",
	}

	store.On("CreateOrUpdateJob", mock.Anything, mock.AnythingOfType("params.Job")).
		Return(params.Job{}, nil).
		Once()
	store.On("UpdateInstance", mock.Anything, "runner-1", mock.MatchedBy(func(update params.UpdateInstanceParams) bool {
		return update.RunnerStatus == params.RunnerActive
	})).
		Return(params.Instance{Name: "runner-1", RunnerStatus: params.RunnerActive}, nil).
		Once()
	store.On("ResetScaleSetCreateFailures", mock.Anything, uint(1)).
		Return(nil).
		Once()

	worker := &Worker{
		ctx:        ctx,
		consumerID: "test-scaleset-budget",
		store:      store,
		scaleSet:   scaleSet,
	}

	err := worker.HandleJobsStarted([]params.ScaleSetJobMessage{{
		MessageType: params.MessageTypeJobStarted,
		JobID:       "job-1",
		RunnerName:  "runner-1",
	}})
	if err != nil {
		t.Fatalf("HandleJobsStarted returned error: %v", err)
	}
}
