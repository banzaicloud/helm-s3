// Copyright © 2021 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"fmt"
	"testing"

	"github.com/minio/minio-go/v6"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	t.Log("Test basic init action")

	const (
		repoName        = "test-init"
		repoDir         = "charts"
		indexObjectName = repoDir + "/index.yaml"
		uri             = "s3://test-init/charts"
	)

	setupBucket(t, repoName)
	defer teardownBucket(t, repoName)

	// Run init.

	cmd, stdout, stderr := command(fmt.Sprintf("helm s3 init %s", uri))
	err := cmd.Run()
	assert.NoError(t, err)
	assertEmptyOutput(t, nil, stderr)
	assert.Contains(t, stdout.String(), "Initialized empty repository at s3://test-init/charts")

	// Check that initialized repository has index file.

	obj, err := mc.StatObject(repoName, indexObjectName, minio.StatObjectOptions{})
	assert.NoError(t, err)
	assert.Equal(t, indexObjectName, obj.Key)

	// Check that `helm repo add` works.

	cmd, stdout, stderr = command(fmt.Sprintf("helm repo add %s %s", repoName, uri))
	err = cmd.Run()
	assert.NoError(t, err)
	assertEmptyOutput(t, nil, stderr)
	assert.Contains(t, stdout.String(), `"test-init" has been added to your repositories`)

	// Check that `helm repo remove` works.

	cmd, stdout, stderr = command(fmt.Sprintf("helm repo remove %s", repoName))
	err = cmd.Run()
	assert.NoError(t, err)
	assertEmptyOutput(t, nil, stderr)
	assert.Contains(t, stdout.String(), `"test-init" has been removed from your repositories`)
}
