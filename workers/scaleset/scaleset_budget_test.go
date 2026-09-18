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
