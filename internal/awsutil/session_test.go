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
)

func TestSessionWithCustomEndpoint(t *testing.T) {
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
