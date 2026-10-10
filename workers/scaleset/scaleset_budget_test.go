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
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"

	providerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbMocks "github.com/cloudbase/garm/database/common/mocks"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	providerMocks "github.com/cloudbase/garm/runner/common/mocks"
)

var budgetTestLockerOnce sync.Once
var budgetTestLockerErr error

func ensureBudgetTestLocker(t *testing.T) {
	t.Helper()
	budgetTestLockerOnce.Do(func() {
		locker, err := locking.NewLocalLocker(context.Background(), nil)
		if err != nil {
			budgetTestLockerErr = err
			return
		}
		budgetTestLockerErr = locking.RegisterLocker(locker)
	})
	if budgetTestLockerErr != nil {
		t.Fatalf("registering local locker: %v", budgetTestLockerErr)
	}
}

func TestProviderListMissNeedsConfirmedAbsence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		getError  error
		confirmed bool
	}{
		{name: "direct get still sees active app", confirmed: false},
		{name: "typed not found", getError: providerErrors.ErrNotFound, confirmed: true},
		{name: "transient provider failure", getError: errors.New("provider unavailable"), confirmed: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := providerMocks.NewProvider(t)
			provider.On("GetInstance", mock.Anything, "provider-runner-1", mock.Anything).
				Return(commonParams.ProviderInstance{Name: "runner-1", ProviderID: "provider-runner-1"}, tc.getError).
				Once()
			worker := &Worker{ctx: context.Background(), provider: provider}
			actual := worker.providerAbsenceConfirmed(params.Instance{
				Name: "runner-1", ProviderID: "provider-runner-1",
			}, common.ProviderBaseParams{})
			if actual != tc.confirmed {
				t.Fatalf("confirmed=%t, want %t", actual, tc.confirmed)
			}
		})
	}
}

func TestConsolidateProviderStateDefersUnconfirmedListMiss(t *testing.T) {
	ensureBudgetTestLocker(t)
	for _, tc := range []struct {
		name     string
		getError error
	}{
		{name: "direct get sees app"},
		{name: "direct get has transient error", getError: errors.New("provider temporarily unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := providerMocks.NewProvider(t)
			provider.On("ListInstances", mock.Anything, "local-truenas-linux-general-11111111-1111-1111-1111-111111111111", mock.Anything).
				Return([]commonParams.ProviderInstance{}, nil).Once()
			provider.On("GetInstance", mock.Anything, "provider-runner-1", mock.Anything).
				Return(commonParams.ProviderInstance{Name: "runner-1", ProviderID: "provider-runner-1"}, tc.getError).Once()
			store := dbMocks.NewStore(t)
			runner := params.Instance{
				ID: "db-1", Name: "runner-1", ProviderID: "provider-runner-1",
				Status: commonParams.InstanceRunning, RunnerStatus: params.RunnerInstalling,
			}
			worker := &Worker{
				ctx: context.Background(), consumerID: "test-list-miss", provider: provider,
				store: store, scaleSet: params.ScaleSet{
					ID: 2, Name: "local-truenas-linux-general",
					RepoID: "11111111-1111-1111-1111-111111111111",
				},
				runners: map[string]params.Instance{runner.ID: runner},
			}
			if err := worker.consolidateProviderState(); err != nil {
				t.Fatalf("consolidating provider state: %v", err)
			}
			if got, ok := worker.runners[runner.ID]; !ok || got.Status != commonParams.InstanceRunning {
				t.Fatalf("runner changed after an unconfirmed list miss: %+v", got)
			}
			// The strict store mock also rejects any DB transition or failure charge.
		})
	}
}

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

	if materializationFailedBeforeActive(params.Instance{
		Status:       commonParams.InstanceError,
		RunnerStatus: params.RunnerPending,
	}) {
		t.Fatal("provider-owned InstanceError must not be double-counted by Scale Set cleanup")
	}
}

func TestHandleJobsStartedResetsMaterializationFailures(t *testing.T) {
	store := dbMocks.NewStore(t)
	ctx := context.Background()
	ensureBudgetTestLocker(t)

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
