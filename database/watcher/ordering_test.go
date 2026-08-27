//go:build testing

// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package watcher_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

func setupWatcher(t *testing.T) func() {
	t.Helper()
	watcher.InitWatcher(context.TODO())
	return func() {
		if w := watcher.GetWatcher(); w != nil {
			w.Close()
			watcher.SetWatcher(nil)
		}
	}
}

func TestEventsAreDeliveredInOrder(t *testing.T) {
	defer setupWatcher(t)()

	ctx := context.TODO()
	prod, err := watcher.RegisterProducer(ctx, "order-producer")
	require.NoError(t, err)
	cons, err := watcher.RegisterConsumer(ctx, "order-consumer")
	require.NoError(t, err)
	defer cons.Close()

	const numEvents = 2000
	done := make(chan error, 1)
	go func() {
		for i := 0; i < numEvents; i++ {
			op := common.UpdateOperation
			if i%2 == 1 {
				op = common.DeleteOperation
			}
			payload := common.ChangePayload{
				EntityType: common.InstanceEntityType,
				Operation:  op,
				Payload: params.Instance{
					Name: fmt.Sprintf("instance-%d", i),
				},
			}
			if err := prod.Notify(payload); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	timeout := time.After(30 * time.Second)
	for i := 0; i < numEvents; i++ {
		select {
		case event := <-cons.Watch():
			instance, ok := event.Payload.(params.Instance)
			require.True(t, ok)
			require.Equal(t, fmt.Sprintf("instance-%d", i), instance.Name, "event %d delivered out of order", i)
			expectedOp := common.UpdateOperation
			if i%2 == 1 {
				expectedOp = common.DeleteOperation
			}
			require.Equal(t, expectedOp, event.Operation)
		case <-timeout:
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
	require.NoError(t, <-done)
}

func TestSlowConsumerDoesNotDropEvents(t *testing.T) {
	defer setupWatcher(t)()

	ctx := context.TODO()
	prod, err := watcher.RegisterProducer(ctx, "burst-producer")
	require.NoError(t, err)
	cons, err := watcher.RegisterConsumer(ctx, "slow-consumer")
	require.NoError(t, err)
	defer cons.Close()

	const numEvents = 200
	for i := 0; i < numEvents; i++ {
		payload := common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.DeleteOperation,
			Payload: params.Instance{
				Name: fmt.Sprintf("instance-%d", i),
			},
		}
		require.NoError(t, prod.Notify(payload))
	}

	timeout := time.After(60 * time.Second)
	for i := 0; i < numEvents; i++ {
		select {
		case event := <-cons.Watch():
			instance, ok := event.Payload.(params.Instance)
			require.True(t, ok)
			require.Equal(t, fmt.Sprintf("instance-%d", i), instance.Name)
		case <-timeout:
			t.Fatalf("timed out waiting for event %d; events were dropped", i)
		}
		time.Sleep(time.Millisecond)
	}
}
