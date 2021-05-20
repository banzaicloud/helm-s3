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
	"net/url"
	"os"

	"emperror.dev/errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	// AwsEndpoint can be set to a custom endpoint to use alternative AWS S3
	// server like minio (https://minio.io).
	awsEndpoint = "AWS_ENDPOINT"

	// AwsDisableSSL can be set to true to disable SSL for AWS S3 server.
	awsDisableSSL = "AWS_DISABLE_SSL"

	// AwsBucketLocation can be set to an AWS region to force the session region
	// if AWS_DEFAULT_REGION and AWS_REGION cannot be trusted.
	awsBucketLocation = "HELM_S3_REGION"
)

// SessionOption is an option for session.
type SessionOption func(*session.Options)

// AssumeRoleTokenProvider is an option for setting custom assume role token provider.
func AssumeRoleTokenProvider(provider func() (string, error)) SessionOption {
	return func(options *session.Options) {
		options.AssumeRoleTokenProvider = provider
	}
}

// DynamicBucketRegion is an option for determining the Helm S3 bucket's AWS
// region dynamically thus allowing the mixed use of buckets residing in
// different regions without requiring manual updates on the HELM_S3_REGION,
// AWS_REGION, or AWS_DEFAULT_REGION environment variables.
//
// This HEAD bucket solution works with all kinds of S3 URIs containing
// the bucket name in the host part.
//
// The basic idea behind the HEAD bucket solution and the "official
// confirmation" this behavior is expected and supported came from a comment on
// the AWS SDK Go repository:
// https://github.com/aws/aws-sdk-go/issues/720#issuecomment-243891223
func DynamicBucketRegion(s3URL string) SessionOption {
	return func(options *session.Options) {
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
			return
		}

		options.Config.Region = aws.String(awsRequest.HTTPResponse.Header[bucketRegionHeader][0])
	}
}

// Session returns an AWS session as described
// http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
func Session(opts ...SessionOption) (awsSession *session.Session, err error) {
	disableSSL := false
	if os.Getenv(awsDisableSSL) == "true" {
		disableSSL = true
	}

	so := session.Options{ // nolint:exhaustivestruct // Note: configuration options.
		Config: aws.Config{ // nolint:exhaustivestruct // Note: configuration options.
			DisableSSL:       aws.Bool(disableSSL),
			S3ForcePathStyle: aws.Bool(true),
			Endpoint:         aws.String(os.Getenv(awsEndpoint)),
		},
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: StderrTokenProvider,
	}

	bucketRegion := os.Getenv(awsBucketLocation)
	// If not set, we don't update the config,
	// so that the AWS SDK can still rely on either AWS_REGION or AWS_DEFAULT_REGION.
	if bucketRegion != "" {
		so.Config.Region = aws.String(bucketRegion)
	}

	for _, opt := range opts {
		opt(&so)
	}

	awsSession, err = session.NewSessionWithOptions(so)
	if err != nil {
		return nil, errors.WrapWithDetails(err, "creating session with options failed, options: %s", so)
	}

	return awsSession, nil
}
