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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"emperror.dev/errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/repo"
)

const (
	// AwsOperationConfirmationKey is the environment key to use to confirm AWS
	// operations.
	awsOperationConfirmationKey string = "HELM_S3_CONFIRM_AWS_OPERATIONS"

	// AwsS3BucketRepositoryPath is the relative path to the Helm S3 repository
	// from the AWS S3 bucket root.
	awsS3BucketRepositoryPath string = "charts"

	// DefaultHelmChartContentType is the default content type of a Helm chart
	// S3 object.
	defaultHelmChartContentType string = "application/gzip"

	// DefaultRegion describes the default region to use when no explicit region is
	// specified. Required for bucket creation.
	defaultRegion string = "eu-central-1"

	// Helmv2StructTag is the struct tag used for Helm major version 2.
	helmv2StructTag string = "helmv2"

	// Helmv3StructTag is the struct tag used for Helm major version 3.
	helmv3StructTag string = "helmv3"

	// LocalStackAccessKeyID is the access key ID for a default configured
	// LocalStack instance.
	localStackAccessKeyID string = "test"

	// LocalStackStatusRunning defines the running status of the LocalStack server.
	localStackStatusRunning localStackStatusType = "running"

	// MakeEndToEndTestEnvironmentSetupRule is the make rule which sets up the
	// end to end test environment.
	makeEndToEndTestEnvironmentSetupRule string = "make setup-e2e-test-env"

	// TestDataRootDirectory is the relative path to the directory containing
	// the generic test data files.
	testDataRootDirectory = "data"
)

// exampleChart returns an example chart.
var exampleChart helmChart = helmChart{ // nolint:gochecknoglobals // Note: intentional.
	Name:        "foo",
	Version:     "1.2.3",
	AppVersion:  "1.2.3",
	Description: "A Helm chart for Kubernetes",
}

// helmChart collects information about a Helm chart known to the Helm binary.
type helmChart struct {
	Name        string `helmv2:"Name" helmv3:"name"`
	Version     string `helmv2:"Version" helmv3:"version"`
	AppVersion  string `helmv2:"AppVersion" helmv3:"app_version"`
	Description string `helmv2:"Description" helmv3:"description"`
}

// helmRepository collects information about a Helm repository known to the Helm
// binary.
type helmRepository struct {
	Name string `helmv2:"Name" helmv3:"name"`
	URL  string `helmv2:"URL" helmv3:"url"`
}

// localStackStatusType collects the possible status values for the LocalStack
// instance.
type localStackStatusType string

// addHelmRepository adds a Helm repository to the local helm cache.
func addHelmRepository(t *testing.T, repositoryName, repositoryURI string) {
	t.Helper()

	require.NotContains(t, listHelmRepositoryNames(t), repositoryName)

	output, errorOutput, err := runCommand("helm", "repo", "add", repositoryName, repositoryURI)
	expectedOutput := fmt.Sprintf("\"%s\" has been added to your repositories\n", repositoryName)
	requireCommandOutput(t, expectedOutput, "", nil, output, errorOutput, err)

	require.Contains(t, listHelmRepositoryNames(t), repositoryName)
}

// checkAWSEnvironment checks whether the test is run in a real AWS environment
// or against Localstack.
func checkAWSEnvironment(t *testing.T, awsConfiguration *aws.Config) {
	t.Helper()

	awsCredentials, err := awsConfiguration.Credentials.Get()
	require.NoError(t, err, "retrieving AWS credentials failed")

	if awsCredentials.AccessKeyID != localStackAccessKeyID &&
		os.Getenv(awsOperationConfirmationKey) != "1" {
		fmt.Printf(
			"WARNING: the AWS access key ID '%s' seems to be a non-LocalStack ID (!= 'test')."+
				"\nTests might execute real AWS calls and create/read/update/delete actual AWS buckets with costs."+
				"\n\nIf you want to run the end to end tests in LocalStack, set its environment up with `%s`."+
				" If you want to use the AWS environment, set the `%s` environment variable to 1 to proceed.\n\n",
			awsCredentials.AccessKeyID,
			makeEndToEndTestEnvironmentSetupRule,
			awsOperationConfirmationKey,
		)

		t.Fatal(
			errors.Errorf(
				"not proceeding with AWS operations without confirmation (`%s`)",
				awsOperationConfirmationKey,
			).Error(),
		)

		return
	}
}

// containsString determines whether the specified collection contains the
// provided string.
func containsString(collection []string, text string) bool {
	for _, item := range collection {
		if item == text {
			return true
		}
	}

	return false
}

// createAWSS3Bucket creates an AWS bucket.
func createAWSS3Bucket(t *testing.T, s3Client *s3.S3, bucketName string) {
	t.Helper()

	require.NotContains(t, listAWSS3Buckets(t, s3Client), bucketName)

	_, err := s3Client.CreateBucket(
		&s3.CreateBucketInput{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
			CreateBucketConfiguration: &s3.CreateBucketConfiguration{
				LocationConstraint: aws.String(defaultRegion),
			},
		})
	require.NoError(t, err, "creating bucket failed, bucket: %s", bucketName)

	require.Contains(t, listAWSS3Buckets(t, s3Client), bucketName)
}

// createDirectory creates a local directory.
func createDirectory(t *testing.T, directoryPath string, mode fs.FileMode) { // nolint:lll // Note: temporary. // Postpone: replace with fs.FileMode at Go 1.18.
	t.Helper()

	require.NoDirExists(t, directoryPath)

	err := os.MkdirAll(directoryPath, mode)
	require.NoError(t, err, "creating directories failed, path: %s", directoryPath)

	require.DirExists(t, directoryPath)
}

// deleteAWSS3Bucket deletes the specified AWS bucket.
func deleteAWSS3Bucket(t *testing.T, s3Client *s3.S3, bucketName string) {
	t.Helper()

	if !containsString(listAWSS3Buckets(t, s3Client), bucketName) {
		return
	}

	deleteAWSS3Objects(t, s3Client, bucketName, listAWSS3ObjectKeys(t, s3Client, bucketName)...)

	_, err := s3Client.DeleteBucket(
		&s3.DeleteBucketInput{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
		},
	)
	require.NoError(t, err, "deleting bucket failed, bucket: %s", bucketName)

	require.NotContains(t, listAWSS3Buckets(t, s3Client), bucketName)
}

// deleteAWSS3Objects deletes the bucket objects behind the specified keys.
func deleteAWSS3Objects(t *testing.T, s3Client *s3.S3, bucketName string, keys ...string) {
	t.Helper()

	if len(keys) == 0 {
		return
	}

	require.Contains(t, listAWSS3Buckets(t, s3Client), bucketName)

	identifiers := make([]*s3.ObjectIdentifier, 0, len(keys))

	for _, key := range keys {
		_ = getAWSS3Object(t, s3Client, bucketName, key)

		identifiers = append(identifiers,
			&s3.ObjectIdentifier{ // nolint:exhaustivestruct // Note: optional query options.
				Key: aws.String(key),
			},
		)
	}

	_, err := s3Client.DeleteObjects(
		&s3.DeleteObjectsInput{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
			Delete: &s3.Delete{ // nolint:exhaustivestruct // Note: optional query values.
				Objects: identifiers,
			},
		},
	)
	require.NoError(t, err, "deleting objects failed, bucket: %s, keys: %+v", bucketName, keys)

	for _, key := range keys {
		getNoAWSS3Object(t, s3Client, bucketName, key)
	}
}

// DeleteDirectory deletes a local directory.
func deleteDirectory(t *testing.T, directoryPath string) {
	t.Helper()

	_, err := os.Stat(directoryPath)
	if err != nil {
		return
	}

	err = os.RemoveAll(directoryPath)
	require.NoError(t, err, "removing directories failed, path: %s", directoryPath)

	require.NoDirExists(t, directoryPath)
}

// deleteFile deletes a local file.
func deleteFile(t *testing.T, filePath string) {
	t.Helper()

	_, err := os.Stat(filePath)
	if err != nil {
		return
	}

	err = os.Remove(filePath)
	require.NoError(t, err, "removing file failed, path: %s", filePath)

	require.NoFileExists(t, filePath)
}

// deleteHelmS3Chart deletes the specified chart from the provided repository.
func deleteHelmS3Chart(t *testing.T, repositoryName, chartName, chartVersion string) {
	t.Helper()

	var chart helmChart

	charts := searchHelmCharts(t, repositoryName, chartName)

	for chartIndex := range charts {
		if charts[chartIndex].Version == chartVersion {
			chart = charts[chartIndex]

			break
		}
	}

	if chart.Name == "" {
		return
	}

	output, errorOutput, err := runCommand(
		"helm", "s3", "delete",
		chart.Name,
		"--version", chart.Version,
		repositoryName,
	)
	requireCommandOutput(t, "", "", nil, output, errorOutput, err)

	require.Empty(t, searchHelmCharts(t, repositoryName, chart.Name))
}

// FetchHelmChart fetches the specified Helm chart.
func fetchHelmChart(t *testing.T, repositoryName, chartName, chartVersion, destination string) {
	t.Helper()

	if destination == "" {
		destination = "."
	}

	require.NoFileExists(t, destination)

	var chart helmChart

	charts := searchHelmCharts(t, repositoryName, chartName)

	for chartIndex := range charts {
		if charts[chartIndex].Version == chartVersion {
			chart = charts[chartIndex]

			break
		}
	}

	require.NotEmpty(t, chart, "chart not found among charts, chart: %+v, charts: %+v", chart, charts)

	_, err := os.Stat(destination)
	if os.IsNotExist(err) {
		createDirectory(t, destination, 0o755) // nolint:gocritic // Note: intentional.
	}

	output, errorOutput, err := runCommand(
		"helm", "fetch",
		path.Join(repositoryName, chartName),
		"--destination", destination,
		"--version", chartVersion,
	)
	requireCommandOutput(t, "", "", nil, output, errorOutput, err)

	require.FileExists(t, path.Join(destination, helmChartFileName(chartName, chartVersion)))
}

// GetAWSS3Object retrieves an AWS S3 bucket object.
func getAWSS3Object(t *testing.T, s3Client *s3.S3, bucketName, objectKey string) *s3.GetObjectOutput {
	t.Helper()

	require.Contains(t, listAWSS3Buckets(t, s3Client), bucketName)

	getObjectOutput, err := s3Client.GetObject(
		&s3.GetObjectInput{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		},
	)
	require.NoError(t, err, "retrieving AWS S3 object failed, bucket: %s, key: %s", bucketName, objectKey)

	return getObjectOutput
}

// getNoAWSS3Object ensures no AWS S3 bucket object can be retrieved with the
// specified key.
func getNoAWSS3Object(t *testing.T, s3Client *s3.S3, bucketName, objectKey string) {
	t.Helper()

	require.Contains(t, listAWSS3Buckets(t, s3Client), bucketName)

	_, err := s3Client.GetObject(
		&s3.GetObjectInput{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		},
	)
	requireAWSErrorCode(t, s3.ErrCodeNoSuchKey, err)
}

// helmChartFileName returns the name for the helm chart file corresponding to
// the specified chart.
func helmChartFileName(chartName, chartVersion string) string {
	return fmt.Sprintf("%s-%s.tgz", chartName, chartVersion)
}

// helmIndexChartVersion ireturns the Helm index's corresponding chart version
// for the specified chart name and provided chart version.
func helmIndexChartVersion(t *testing.T, indexPath, chartName, chartVersion string) *repo.ChartVersion {
	t.Helper()

	index, err := repo.LoadIndexFile(indexPath)
	require.NoError(t, err, "loading index file failed, path: %s", indexPath)

	indexChartVersion, err := index.Get(chartName, chartVersion)
	require.NoError(
		t,
		err,
		"retrieving chert version from index failed, chart: %s, version: %s, index: %s",
		chartName, chartVersion, index.Entries,
	)

	return indexChartVersion
}

// helmS3RepositoryChartPath returns the relative path to the specified chart on
// the Helm repository. Existence of the chart is not ensured.
func helmS3RepositoryChartPath(chartName, chartVersion string) string {
	return helmS3RepositoryFilePath(helmChartFileName(chartName, chartVersion))
}

// helmS3RepositoryFilePath returns the relative path to the Helm repository's
// file with the specified name.
func helmS3RepositoryFilePath(fileName string) string {
	return path.Join(awsS3BucketRepositoryPath, fileName)
}

// helmS3RepositoryURI returns the URI of the Helm repository corresponding to
// the specified bucket name and repository subpath..
func helmS3RepositoryURI(bucketName string) string {
	return fmt.Sprintf("s3://%s/%s", bucketName, awsS3BucketRepositoryPath)
}

// helmSearchCommand returns the Helm search command based on the Helm major
// version.
func helmSearchCommand(t *testing.T) []string {
	t.Helper()

	version := helmVersion(t)

	switch {
	case strings.HasPrefix(version, "v2."):
		return []string{"helm", "search"}
	case strings.HasPrefix(version, "v3."):
		return []string{"helm", "search", "repo"}
	default:
		t.Fatalf("unsupported Helm version, version: %s", version)

		return nil // Note: for compiler code analysis.
	}
}

// helmStructTag returns the struct tag for the current Helm binary version.
func helmStructTag(t *testing.T) string {
	t.Helper()

	version := helmVersion(t)

	switch {
	case strings.HasPrefix(version, "v2."):
		return helmv2StructTag
	case strings.HasPrefix(version, "v3."):
		return helmv3StructTag
	default:
		t.Fatalf("unsupported Helm version, version: %s", version)

		return "" // Note: for compiler code analysis.
	}
}

// helmVersion returns the version of the Helm binary.
func helmVersion(t *testing.T) string {
	t.Helper()

	versionRawRegexp := `^.+:"(v?[0-9]+\.[0-9]+\.[0-9]+)".+`
	versionRegexp, err := regexp.Compile(versionRawRegexp)
	require.NoError(t, err, "raw regular expression: %s", versionRawRegexp)

	output, errorOutput, err := runCommand("helm", "version", "--client")
	require.NoError(t, err, "output: %s, errorOutput: %s", output, errorOutput)
	require.Empty(t, errorOutput, "output: %s", output)

	groups := versionRegexp.FindStringSubmatch(output)
	if len(groups) < 2 {
		t.Fatalf("Helm version cannot be determined, version output: %s", output)
	}

	return groups[1]
}

// initializeAWSEnvironment initializes the AWS environment. (AWS credentials,
// endpoint, region are required for Helm.)
func initializeAWSEnvironment(t *testing.T, awsConfiguration *aws.Config) {
	t.Helper()

	awsCredentials, err := awsConfiguration.Credentials.Get()
	require.NoError(t, err, "retrieving AWS credentials failed")

	err = os.Setenv("AWS_ACCESS_KEY_ID", awsCredentials.AccessKeyID)
	require.NoError(t, err, "setting environment variable AWS_ACCESS_KEY_ID failed")

	if awsConfiguration.Endpoint != nil {
		err = os.Setenv("AWS_ENDPOINT", aws.StringValue(awsConfiguration.Endpoint))
		require.NoError(t, err, "setting environment variable AWS_ENDPOINT failed")
	}

	if awsConfiguration.Region != nil {
		err = os.Setenv("AWS_REGION", aws.StringValue(awsConfiguration.Region))
		require.NoError(t, err, "setting environment variable AWS_REGION failed")
	}

	err = os.Setenv("AWS_SECRET_ACCESS_KEY", awsCredentials.SecretAccessKey)
	require.NoError(t, err, "setting environment variable AWS_SECRET_ACCESS_KEY failed")
}

// initializeHelmEnvironment initializes the Helm environment. (Helm repo list
// requires an existing repository config file even for formatted output to
// return no error on empty repository list.)
func initializeHelmEnvironment(t *testing.T) {
	t.Helper()

	version := helmVersion(t)

	switch {
	case strings.HasPrefix(version, "v2."):
		output, errorOutput, err := runCommand("helm", "init", "--client-only")
		require.NoError(t, err, "output: %s, errorOutput: %s", output, errorOutput)
		require.Empty(t, errorOutput, "output: %s", output)
	case strings.HasPrefix(version, "v3."):
		output, errorOutput, err := runCommand("helm", "env", "HELM_REPOSITORY_CONFIG")
		require.NoError(t, err, "output: %s, errorOutput: %s", output, errorOutput)

		_, err = os.Stat(output)
		if os.IsNotExist(err) {
			expectedAddedOutput := "\"stable\" has been added to your repositories\n"
			expectedAlreadyExistsOutput := "\"stable\" already exists with the same configuration, skipping\n"
			expectedRemovedOutput := "\"stable\" has been removed from your repositories\n"

			output, errorOutput, err = runCommand("helm", "repo", "add", "stable", "https://charts.helm.sh/stable")
			require.NoError(t, err, "output: %s, errorOutput: %s")
			require.Equal(t, "", errorOutput)
			require.Contains(t, []string{expectedAddedOutput, expectedAlreadyExistsOutput}, output, "output: %s", output)

			if output == expectedAddedOutput {
				output, errorOutput, err = runCommand("helm", "repo", "remove", "stable")
				requireCommandOutput(t, expectedRemovedOutput, "", nil, output, errorOutput, err)
			}
		}
	default:
		t.Fatalf("invalid Helm version, Helm version: %s", version)
	}
}

// initializeHelmS3Repository initializes an AWS S3 Helm repository at the
// specified URI.
func initializeHelmS3Repository(t *testing.T, s3Client *s3.S3, bucketName, repositoryURI string) {
	t.Helper()

	require.Len(t, listAWSS3ObjectKeys(t, s3Client, bucketName), 0)

	output, errorOutput, err := runCommand("helm", "s3", "init", repositoryURI)
	expectedOutput := fmt.Sprintf("Initialized empty repository at %s\n", repositoryURI)
	requireCommandOutput(t, expectedOutput, "", nil, output, errorOutput, err)

	_ = getAWSS3Object(t, s3Client, bucketName, helmS3RepositoryFilePath("index.yaml"))
}

// listAWSS3Buckets returns the list of AWS buckets.
func listAWSS3Buckets(t *testing.T, s3Client *s3.S3) []string {
	t.Helper()

	listBucketsOutput, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	require.NoError(t, err, "listing buckets failed")

	buckets := make([]string, 0, len(listBucketsOutput.Buckets))
	for _, bucket := range listBucketsOutput.Buckets {
		buckets = append(buckets, aws.StringValue(bucket.Name))
	}

	return buckets
}

// listAWSS3ObjectKeys returns the specified bucket's AWS bucket object keys.
func listAWSS3ObjectKeys(t *testing.T, s3Client *s3.S3, bucketName string) []string {
	t.Helper()

	require.Contains(t, listAWSS3Buckets(t, s3Client), bucketName)

	listObjectsOutput, err := s3Client.ListObjectsV2(
		&s3.ListObjectsV2Input{ // nolint:exhaustivestruct // Note: optional query values.
			Bucket: aws.String(bucketName),
		},
	)
	require.NoError(t, err, "listing objects failed, bucket: %s", bucketName)

	objectKeys := make([]string, 0, len(listObjectsOutput.Contents))
	for _, object := range listObjectsOutput.Contents {
		objectKeys = append(objectKeys, aws.StringValue(object.Key))
	}

	return objectKeys
}

// listHelmRepositoryNames returns the names of the Helm repositories.
func listHelmRepositoryNames(t *testing.T) []string {
	t.Helper()

	output, errorOutput, err := runCommand("helm", "repo", "list", "--output", "yaml")
	require.NoError(t, err, "output: %s, errorOutput: %s", output, errorOutput)

	var yamlOutput interface{}
	err = yaml.Unmarshal([]byte(output), &yamlOutput)
	require.NoError(t, err, "parsing Helm repo list YAML failed, YAML: %s", output)

	var repositories []helmRepository

	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook:       nil,
		ErrorUnused:      true,
		ZeroFields:       true,
		WeaklyTypedInput: false,
		Squash:           true,
		Metadata:         nil,
		Result:           &repositories,
		TagName:          helmStructTag(t),
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	require.NoError(t, err, "creating Helm repo list YAML decoder failed, config: %+v", decoderConfig)

	err = decoder.Decode(yamlOutput)
	require.NoError(t, err, "decoding Helm repo list YAML failed, YAML: %s, config: %s", yamlOutput, decoderConfig)

	repositoryNames := make([]string, 0, len(repositories))

	for repositoryIndex := range repositories {
		repositoryNames = append(repositoryNames, repositories[repositoryIndex].Name)
	}

	return repositoryNames
}

// localStackStatus returns the current status of the LocalStack instance
// behind the specified URL or alternatively an error.
func localStackStatus(localStackURL string) (localStackStatusType, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, localStackURL, nil)
	if err != nil {
		return "", errors.WrapWithDetails(
			err,
			"creating LocalStack check request failed",
			"localStackURL", localStackURL,
		)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", errors.WrapWithDetails(err, "checking LocalStack request failed", "localStackURL", localStackURL)
	} else if response == nil {
		return "", errors.WithDetails(
			errors.Errorf("receiving LocalStack check response failed"),
			"localStackURL", localStackURL,
		)
	}

	defer func() { _ = response.Body.Close() }()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		responseDump, _ := httputil.DumpResponse(response, true)

		return "", errors.WrapWithDetails(
			err,
			"reading LocalStack check response failed",
			"localStackURL", localStackURL,
			"response", string(responseDump),
		)
	}

	var parsedData map[string]interface{}
	if err = json.Unmarshal(data, &parsedData); err != nil {
		return "", errors.WrapWithDetails(
			err,
			"parsing LocalStack check response data failed",
			"localStackURL", localStackURL,
			"data", string(data),
		)
	}

	stringStatus, isOk := parsedData["status"].(string)
	if !isOk {
		return "", errors.WithDetails(
			errors.Errorf("status field is not a string"),
			"status", parsedData["status"],
			"localStackURL", localStackURL,
		)
	}

	return localStackStatusType(stringStatus), nil
}

// newTestAWSConfiguration initializes an AWS configuration context for testing
// purposes based on the executing environment.
func newAWSConfiguration() *aws.Config { // nolint:funlen // Note: easier to understand.
	awsCredentialsFilePath, _ := newFirstStringValueAdapter(
		newEnvironmentStringValueAdapter("AWS_SHARED_CREDENTIALS_FILE"),
		newFilePathStringValueAdapter(filepath.Join(os.Getenv("HOME"), ".aws", "credentials")),
	).StringValue()

	awsEndpoint, hasAWSEndpoint := newFirstStringValueAdapter(
		newEnvironmentStringValueAdapter("AWS_ENDPOINT"),
		newEnvironmentStringValueAdapter("LOCALSTACK_AWS_ENDPOINT"),
		newDynamicStringValueAdapter(func() (string, bool) {
			defaultURL := "http://s3.localhost.localstack.cloud:4566" // Note: for AWS S3 HEAD bucket.
			if status, err := localStackStatus(defaultURL); err != nil || status != localStackStatusRunning {
				return "", false
			}

			return defaultURL, true
		}),
	).StringValue()

	isUsingLocalStack := false
	if status, err := localStackStatus(awsEndpoint); err == nil && status == localStackStatusRunning {
		isUsingLocalStack = true
	}

	awsLogLevel, hasAWSLogLevel := newFirstAWSLogLevelValueAdapter(
		newEnvironmentAWSLogLevelValueAdapter("AWS_LOG_LEVEL"),
		newEnvironmentAWSLogLevelValueAdapter("LOCALSTACK_AWS_LOG_LEVEL"),
		newDynamicAWSLogLevelValueAdapter(func() (aws.LogLevelType, bool) {
			_, isSet := os.LookupEnv("DEBUG")
			if !isSet {
				return aws.LogOff, false
			}

			return aws.LogDebugWithHTTPBody, true
		}),
		newStaticAWSLogLevelValueAdapter(aws.LogOff),
	).AWSLogLevelValue()

	awsProfile, _ := newFirstStringValueAdapter(
		newEnvironmentStringValueAdapter("AWS_PROFILE"),
		newEnvironmentStringValueAdapter("LOCALSTACK_AWS_PROFILE"),
		newStaticStringValueAdapter("default"),
	).StringValue()

	awsRegion, hasAWSRegion := newFirstStringValueAdapter(
		newEnvironmentStringValueAdapter("HELM_S3_REGION"),
		newEnvironmentStringValueAdapter("AWS_REGION"),
		newEnvironmentStringValueAdapter("AWS_DEFAULT_REGION"),
		newEnvironmentStringValueAdapter("LOCALSTACK_AWS_REGION"),
		newEnvironmentStringValueAdapter("LOCALSTACK_AWS_DEFAULT_REGION"),
		newConfigurationFileStringValueAdapter("region", awsProfile),
		newStaticStringValueAdapter("us-east-1"),
	).StringValue()

	awsRoleARN, _ := newFirstStringValueAdapter(
		newConfigurationFileStringValueAdapter("role_arn", awsProfile),
	).StringValue()

	awsRoleARNWebIdentity, _ := newFirstStringValueAdapter(
		newEnvironmentStringValueAdapter("AWS_ROLE_ARN"),
	).StringValue()

	awsWebIdentityTokenFilePath, _ := newFirstStringValueAdapter(
		newConfigurationFileStringValueAdapter("web_identity_token_file", awsProfile),
	).StringValue()

	var awsCredentialsProviders []credentials.Provider
	if isUsingLocalStack {
		awsCredentialsProviders = []credentials.Provider{
			newConditionalAWSCredentialsProvider(
				isUsingLocalStack,
				&credentials.StaticProvider{
					Value: credentials.Value{
						AccessKeyID:     "test",
						SecretAccessKey: "test",
						SessionToken:    "",
						ProviderName:    "",
					},
				},
			),
		}
	} else {
		awsCredentialsProviders = newNotNilAWSCredentialsProviders(
			newConditionalAWSCredentialsProvider(!isUsingLocalStack, &credentials.EnvProvider{}),
			newConditionalAWSCredentialsProvider(
				awsCredentialsFilePath != "",
				&credentials.SharedCredentialsProvider{
					Filename: awsCredentialsFilePath,
					Profile:  awsProfile,
				},
			),
			newConditionalAWSCredentialsProvider(
				awsRoleARN != "",
				&stscreds.AssumeRoleProvider{ // nolint:exhaustivestruct // Note: complex structure.
					Client:        sts.New(session.Must(session.NewSession())),
					RoleARN:       awsRoleARN,
					Duration:      stscreds.DefaultDuration,
					TokenProvider: stscreds.StdinTokenProvider,
				},
			),
			newConditionalAWSCredentialsProvider(
				awsRoleARNWebIdentity != "" && awsWebIdentityTokenFilePath != "",
				stscreds.NewWebIdentityRoleProvider(
					sts.New(session.Must(session.NewSession())),
					awsRoleARNWebIdentity,
					"helm-s3-end-to-end-test-"+time.Now().Format(time.RFC3339Nano),
					awsWebIdentityTokenFilePath,
				),
			),
		)
	}

	awsConfiguration := aws.NewConfig().
		WithCredentials(credentials.NewChainCredentials(awsCredentialsProviders))

	if hasAWSEndpoint {
		awsConfiguration = awsConfiguration.WithEndpoint(awsEndpoint)
	}

	if hasAWSLogLevel {
		awsConfiguration = awsConfiguration.WithLogLevel(awsLogLevel)
	}

	if hasAWSRegion {
		awsConfiguration = awsConfiguration.WithRegion(awsRegion)
	}

	return awsConfiguration
}

// newConditionalAWSCredentialsProvider returns the specified provider if the
// condition evaluates to true, otherwise it returns nil.
func newConditionalAWSCredentialsProvider(condition bool, provider credentials.Provider) credentials.Provider {
	if !condition {
		return nil
	}

	return provider
}

// newNotNilAWSCredentialsProviders returns a collection of credentials
// providers based on the specified providers, excluding nil providers.
func newNotNilAWSCredentialsProviders(providers ...credentials.Provider) []credentials.Provider {
	notNilProviders := make([]credentials.Provider, 0, len(providers))

	for _, provider := range providers {
		if provider != nil {
			notNilProviders = append(notNilProviders, provider)
		}
	}

	return notNilProviders
}

// newUniqueBucketName tries to create a universally unique bucket name using the
// specified prefix.
func newUniqueBucketName(prefixes ...string) string {
	name := strings.Join(append(prefixes, uuid.New().String()), "-")
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

// pushHelmS3Chart pushes the specified chart to the provided repository.
func pushHelmS3Chart(t *testing.T, repositoryName, chartPath string, options ...string) {
	t.Helper()

	output, errorOutput, err := tryPushHelmS3Chart(repositoryName, chartPath, options...)
	requireCommandOutput(t, "", "", nil, output, errorOutput, err)
}

// reindexHelmS3 reindexes the Helm S3 repository.
func reindexHelmS3(t *testing.T, repositoryName string) {
	t.Helper()

	output, errorOutput, err := runCommand("helm", "s3", "reindex", repositoryName)
	expectedOutput := fmt.Sprintf("Repository %s was successfully reindexed.\n", repositoryName)
	requireCommandOutput(t, expectedOutput, "", nil, output, errorOutput, err)
}

// removeHelmRepository removes the specified Helm S3 repository.
func removeHelmRepository(t *testing.T, repositoryName string) {
	t.Helper()

	repositoryNames := listHelmRepositoryNames(t)
	if !containsString(repositoryNames, repositoryName) {
		return
	}

	output, errorOutput, err := runCommand("helm", "repo", "remove", repositoryName)
	expectedOutput := fmt.Sprintf("\"%s\" has been removed from your repositories\n", repositoryName)
	requireCommandOutput(t, expectedOutput, "", nil, output, errorOutput, err)

	repositoryNames = listHelmRepositoryNames(t)
	require.NotContains(t, repositoryNames, repositoryName)
}

// runCommand runs the specified command with the provided arguments and returns
// its result.
func runCommand(commandAndArguments ...string) (output, errorOutput string, err error) {
	if len(commandAndArguments) == 0 {
		return "", "", errors.Errorf("missing required command argument")
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	cmd := exec.Command( // nolint:gosec // Note: reported only for audit purposes.
		commandAndArguments[0],
		commandAndArguments[1:]...,
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// saveAWSS3ObjectLocally saves the specified object locally to the provided
// path.
func saveAWSS3ObjectLocally(t *testing.T, object *s3.GetObjectOutput, filePath string, mode fs.FileMode) { // nolint:lll // Note: temporary. // Postpone: replace with fs.FileMode at Go 1.18.
	t.Helper()

	data, err := io.ReadAll(object.Body)
	require.NoError(t, err, "reading AWS object body failed, local path: %s", filePath)

	_, err = os.Stat(path.Dir(filePath))
	if os.IsNotExist(err) {
		createDirectory(t, path.Dir(filePath), 0o755) // nolint:gocritic // Note: intentional.
	}

	err = os.WriteFile(filePath, data, mode)
	require.NoError(t, err, "writing file failed, path: %s", filePath)
}

// SearchHelmCharts returns the corresponding chart to the specified repository
// and chart name if it can be found.
func searchHelmCharts(t *testing.T, repositoryName, chartName string) []helmChart {
	t.Helper()

	updateHelmRepositories(t)

	output, errorOutput, err := runCommand(
		append(helmSearchCommand(t), path.Join(repositoryName, chartName), "--output", "yaml")...,
	)
	require.NoError(t, err, "output: %s, error output: %s", output, errorOutput)

	var yamlOutput interface{}
	err = yaml.Unmarshal([]byte(output), &yamlOutput)
	require.NoError(t, err, "parsing Helm search repo YAML failed, YAML: %s", output)

	var charts []helmChart

	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook:       nil,
		ErrorUnused:      true,
		ZeroFields:       true,
		WeaklyTypedInput: false,
		Squash:           true,
		Metadata:         nil,
		Result:           &charts,
		TagName:          helmStructTag(t),
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	require.NoError(t, err, "creating Helm search repo YAML decoder failed, config: %+v", decoderConfig)

	err = decoder.Decode(yamlOutput)
	require.NoError(t, err, "decoding Helm search repo YAML failed, YAML: %s, config: %s", yamlOutput, decoderConfig)

	for chartIndex := range charts {
		charts[chartIndex].Name = path.Base(charts[chartIndex].Name) // Note: removing repository prefix.
	}

	return charts
}

// setHelmS3Region sets the HELM_S3_REGION environment variable to the specified
// value.
func setHelmS3Region(t *testing.T, value string) {
	t.Helper()

	err := os.Setenv("HELM_S3_REGION", value)
	require.NoError(t, err, "setting HELM_S3_REGION failed, value: %s", value)
}

// temporaryDirectoryPath returns a temporary directory path for the specified
// path elements.
func temporaryDirectoryPath(pathElements ...string) string {
	return path.Join(append([]string{os.TempDir(), "helm-s3"}, pathElements...)...)
}

// testChartPath returns a path to the specified helm chart's local test chart
// package file.
func testChartPath(t *testing.T, chartName, chartVersion string) string {
	t.Helper()

	chartPath := path.Join(testDataRootDirectory, helmChartFileName(chartName, chartVersion))
	require.FileExists(t, chartPath)

	return chartPath
}

// toLowerWordsFromCamelOrPascalCase returns the collection of words from a
// camel or Pascal cased text.
//
// WARNING: this fails on joint acronym expressions like HTTPAPI (becomes
// []string{"httpapi"}), because the word boundary cannot be determined without
// contextual knowledge, but otherwise is a good approximation.
func toLowerWordsFromCamelOrPascalCase(text string) []string {
	words := make([]string, 0, 4)

	lastWord := ""
	lastCharacter := 'A'

	for _, character := range text {
		if unicode.IsUpper(character) &&
			!unicode.IsUpper(lastCharacter) {
			words = append(words, lastWord)
			lastWord = ""
		}

		lastWord += string(unicode.ToLower(character))
		lastCharacter = character
	}

	words = append(words, lastWord)

	return words
}

// tryPushHelmS3Chart attempts to push the specified chart to the provided
// repository and returns the result.
func tryPushHelmS3Chart(repositoryName, chartPath string, options ...string) (output, errorOutput string, err error) {
	return runCommand(append([]string{"helm", "s3", "push", chartPath, repositoryName}, options...)...)
}

// updateHelmRepositories updates the known Helm repositories in the local cache.
func updateHelmRepositories(t *testing.T) {
	t.Helper()

	output, errorOutput, err := runCommand("helm", "repo", "update")
	require.NoError(t, err, "output: %s, errorOutput: %s", output, errorOutput)
}
