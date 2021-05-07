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
	"testing"

	"emperror.dev/errors"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/require"
)

// requireAWSErrorCode extracts the AWS error code from the specified actual
// error and compares it to the expected value.
func requireAWSErrorCode(t *testing.T, expectedAWSErrorCode string, actualError error) {
	t.Helper()

	require.Error(t, actualError, "expected AWS error, received nil, expected AWS error code: %s", expectedAWSErrorCode)

	var actualAWSError awserr.Error
	isOk := errors.As(actualError, &actualAWSError)
	require.True(t, isOk, "actual error is not an AWS error, actual error: %s", actualError)

	require.Equal(t, expectedAWSErrorCode, actualAWSError.Code(), "unexpected AWS error: %s", actualAWSError)
}

// requireCommandOutput compares expected and actual outputs of a command.
func requireCommandOutput(
	t *testing.T,
	expectedOutput string,
	expectedErrorOutput string,
	expectedError error,
	actualOutput string,
	actualErrorOutput string,
	actualError error,
) {
	t.Helper()

	if expectedError != nil {
		errorMessage := expectedError.Error()
		require.EqualError(t, actualError, errorMessage, "output: %s, errorOutput: %s", actualOutput, actualErrorOutput)
	} else {
		require.NoError(t, actualError, "output: %s, errorOutput: %s", actualOutput, actualErrorOutput)
	}

	require.Equal(t, expectedErrorOutput, actualErrorOutput, "output: %s", actualOutput)
	require.Equal(t, expectedOutput, actualOutput)
}
