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

package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	s3URL := "s3://eu-central-1-test-bucket"

	if len(os.Args) > 1 {
		s3URL = os.Args[1]
	}

	parsedS3URL, err := url.Parse(s3URL)
	if err != nil {
		return
	}

	// Note: The dummy credentials are required in case no other credential
	// provider is found, but even if the HEAD bucket request fails and
	// returns a non-200 status code indicating no access to the bucket, the
	// actual bucket region is returned in a response header.
	//
	// Note: A signing region **MUST** be configured, otherwise the signed
	// request fails. The configured region itself is irrelevant, the
	// endpoint officially works and returns the bucket region in a response
	// header regardless of whether the signing region matches the bucket's
	// region.
	//
	// Note: The default S3 endpoint **MUST** be configured to avoid making
	// the request region specific thus avoiding regional redirect responses
	// (301 Permanently moved) on HEAD bucket. This setting is only required
	// because any other region than "us-east-1" would configure a
	// region-specific endpoint as well, so it's more safe to explicitly
	// configure the default endpoint.
	//
	// Source:
	// https://github.com/aws/aws-sdk-go/issues/720#issuecomment-243891223
	configuration := aws.NewConfig().
		WithLogLevel(aws.LogDebugWithHTTPBody).
		WithCredentials(credentials.NewStaticCredentials("dummy", "dummy", "")).
		WithRegion("us-east-1").
		WithEndpoint("s3.amazonaws.com")
	awsSession := session.Must(session.NewSession())
	s3Client := s3.New(awsSession, configuration)

	bucketRegionHeader := "X-Amz-Bucket-Region"
	input := &s3.HeadBucketInput{ // nolint:exhaustivestruct // Note: optional query elements.
		Bucket: aws.String(parsedS3URL.Host),
	}
	awsRequest, _ := s3Client.HeadBucketRequest(input)
	_ = awsRequest.Send()

	if awsRequest.HTTPResponse == nil ||
		len(awsRequest.HTTPResponse.Header[bucketRegionHeader]) == 0 {
		fmt.Printf("Error: no bucket region header found for %s\n", s3URL)

		os.Exit(1)
	}

	fmt.Printf("Region: %s\n", awsRequest.HTTPResponse.Header[bucketRegionHeader][0])
}
