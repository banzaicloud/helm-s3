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

package awsutil

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestDynamicBucketRegion(t *testing.T) { 
	t.Parallel()

	defaultSession, err := Session()
	require.NoError(t, err)

	defaultRegion := aws.StringValue(defaultSession.Config.Region)

	testCases := []struct {
		caseDescription      string
		expectedBucketRegion string
		inputS3URL           string
	}{
		{
			caseDescription:      "existing S3 bucket URL with host only (no key) -> success",
			expectedBucketRegion: "eu-central-1",
			inputS3URL:           "s3://eu-test-bucket",
		},
		{
			caseDescription:      "existing S3 bucket URL with key -> success",
			expectedBucketRegion: "ap-southeast-2",
			inputS3URL:           "s3://cn-test-bucket/charts/chart-0.1.2.tgz",
		},
		{
			caseDescription:      "invalid URL -> failing URI parsing, no effect (default region)",
			expectedBucketRegion: defaultRegion,
			inputS3URL:           "://not/a/URL",
		},
		{
			caseDescription:      "invalid S3 URL -> failing request, no effect (default region)",
			expectedBucketRegion: defaultRegion,
			inputS3URL:           "",
		},
		{
			caseDescription:      "not existing S3 URL -> no region header, no effect (default region)",
			expectedBucketRegion: defaultRegion,
			inputS3URL:           "s3://not-an-s3-bucket-url",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.caseDescription, func(t *testing.T) {
			t.Parallel()

			actualSession, err := Session(DynamicBucketRegion(testCase.inputS3URL))
			require.NoError(t, err, "creating session failed")
			require.Equal(t, testCase.expectedBucketRegion, aws.StringValue(actualSession.Config.Region))
		})
	}
}

func TestSessionWithCustomEndpoint(t *testing.T) { // nolint:paralleltest // Note: requires refactor.
	os.Setenv("AWS_ENDPOINT", "foobar:1234")
	os.Setenv("AWS_DISABLE_SSL", "true")
	os.Setenv("HELM_S3_REGION", "us-west-2")

	s, err := Session()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if *s.Config.Endpoint != "foobar:1234" {
		t.Fatalf("Expected endpoint to be foobar:1234")
	}

	if !*s.Config.DisableSSL {
		t.Fatalf("Expected to disable SSL")
	}

	if *s.Config.Region != "us-west-2" {
		t.Fatalf("Expected to set us-west-2 region")
	}

	os.Unsetenv("AWS_ENDPOINT")
	os.Unsetenv("AWS_DISABLE_SSL")
	os.Unsetenv("HELM_S3_REGION")
}
