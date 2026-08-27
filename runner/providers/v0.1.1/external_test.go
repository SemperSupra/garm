// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package v011

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/runner/common"
)

func writeTestProvider(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test provider uses a POSIX shell script")
	}

	path := filepath.Join(t.TempDir(), "provider.sh")
	content := "#!/bin/sh\nset -eu\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func testExternalProvider(path string) *external {
	return &external{
		cfg: &config.Provider{
			Name: "test-provider",
		},
		controllerID: "test-controller",
		execPath:     path,
	}
}

func TestListInstancesAcceptsSuccessfulProviderResponse(t *testing.T) {
	provider := testExternalProvider(writeTestProvider(t, `
printf '%s\n' '[{"provider_id":"provider-1","name":"runner-1","status":"running"}]'
`))

	instances, err := provider.ListInstances(context.Background(), "pool-1", common.ListInstancesParams{})
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, "provider-1", instances[0].ProviderID)
	require.Equal(t, "runner-1", instances[0].Name)
	require.Equal(t, "running", string(instances[0].Status))
}

func TestListInstancesReturnsProviderExecutionErrorBeforeDecode(t *testing.T) {
	provider := testExternalProvider(writeTestProvider(t, `
printf '%s\n' 'not-json'
exit 17
`))

	instances, err := provider.ListInstances(context.Background(), "pool-1", common.ListInstancesParams{})
	require.Error(t, err)
	require.Empty(t, instances)
	require.Contains(t, err.Error(), "provider binary")
	require.Contains(t, err.Error(), "returned error")
	require.NotContains(t, err.Error(), "failed to decode response")
}
