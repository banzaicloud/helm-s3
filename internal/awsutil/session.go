package awsutil

import (
	"log"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	// awsEndpoint can be set to a custom endpoint to use alternative AWS S3
	// server like minio (https://minio.io).
	awsEndpoint = "AWS_ENDPOINT"

	// awsDisableSSL can be set to true to disable SSL for AWS S3 server.
	awsDisableSSL = "AWS_DISABLE_SSL"

	// awsBucketLocation can be set to an AWS region to force the session region
	// if AWS_DEFAULT_REGION and AWS_REGION cannot be trusted
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
// different regions without the need for  manual updating
// HELM_S3_REGION/AWS_REGION/AWS_DEFAULT_REGION.
//
// As the bucket URI will always be using the S3 protocol, this is going to work
// for S3 bucket chart repositories. THis would not work for HTTPS proxied
// buckets, because those don't respond to the HEAD request as expected, but
// that's fine, because the plugin only handles S3 protocols, the regular helm
// binary can handle HTTPS repositories in such a case.
func DynamicBucketRegion(bucketURI string) SessionOption {
	return func(options *session.Options) {
		parsedBucketURI, err := url.Parse(bucketURI)
		if err != nil {
			log.Printf("[WARNING] parsing bucket URI for dynamic bucket region failed, invalid URI: %s\n", bucketURI)

			return
		}

		// Note: the configured region itself is irrelevant, the endpoint
		// officially works and returns the bucket region in a response header
		// regardless of whether the signing region matches the bucket's region,
		//
		// Note: the credentials are also irrelevant, because even if the HEAD
		// bucket request fails and returns non-200 status code indicating no
		// access to the bucket, the actual bucket region is returned in a
		// response header.
		//
		// Source:
		// https://github.com/aws/aws-sdk-go/issues/720#issuecomment-243891223.
		configuration := aws.NewConfig().
			WithRegion("us-east-1")
		session := session.Must(session.NewSession())
		s3Client := s3.New(session, configuration)

		bucketRegionHeader := "X-Amz-Bucket-Region"
		input := &s3.HeadBucketInput{
			Bucket: aws.String(parsedBucketURI.Host),
		}
		request, _ := s3Client.HeadBucketRequest(input)
		err = request.Send()
		if request.HTTPResponse == nil { // Note: only the header part of the response is relevant.
			requestDump, _ := httputil.DumpRequest(request.HTTPRequest, false)
			log.Printf(
				"[WARNING] requesting dynamic bucket region through HEAD bucket failed: %s, request: %s\n",
				err.Error(),
				string(requestDump),
			)

			return
		} else if len(request.HTTPResponse.Header[bucketRegionHeader]) == 0 {
			requestDump, _ := httputil.DumpRequest(request.HTTPRequest, false)
			responseDump, _ := httputil.DumpResponse(request.HTTPResponse, false)
			log.Printf(
				"[WARNING] dynamic bucket region header not found in the HEAD bucket response, response: %s\n, request: %s",
				string(responseDump),
				string(requestDump),
			)

			return
		}

		options.Config.Region = aws.String(request.HTTPResponse.Header[bucketRegionHeader][0])
	}
}

// Session returns an AWS session as described http://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
func Session(opts ...SessionOption) (*session.Session, error) {
	disableSSL := false
	if os.Getenv(awsDisableSSL) == "true" {
		disableSSL = true
	}

	so := session.Options{
		Config: aws.Config{
			DisableSSL:       aws.Bool(disableSSL),
			S3ForcePathStyle: aws.Bool(true),
			Endpoint:         aws.String(os.Getenv(awsEndpoint)),
		},
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: StderrTokenProvider,
	}

	bucketRegion := os.Getenv(awsBucketLocation)
	// if not set, we don't update the config,
	// so that the AWS SDK can still rely on either AWS_REGION or AWS_DEFAULT_REGION
	if bucketRegion != "" {
		so.Config.Region = aws.String(bucketRegion)
	}

	for _, opt := range opts {
		opt(&so)
	}

	return session.NewSessionWithOptions(so)
}
