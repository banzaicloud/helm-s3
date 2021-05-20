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
	"path"
	"time"

	"emperror.dev/errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func (testSuite *EndToEndSuite) TestHelmS3Push() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName
	temporaryLocalChartPath := testSuite.TemporaryPath(testName, helmChartFileName(chart.Name, chart.Version))

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath)

	object := getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	require.Equal(testSuite.T(), defaultHelmChartContentType, aws.StringValue(object.ContentType))

	require.Contains(testSuite.T(), searchHelmCharts(testSuite.T(), repositoryName, chart.Name), chart)

	fetchHelmChart(testSuite.T(), repositoryName, chart.Name, chart.Version, path.Dir(temporaryLocalChartPath))
	require.FileExists(testSuite.T(), temporaryLocalChartPath)

	output, errorOutput, err := tryPushHelmS3Chart(repositoryName, localChartPath)
	expectedErrorOutput :=
		"The chart already exists in the repository and cannot be overwritten without an explicit intent." +
			" If you want to replace existing chart, use --force flag:" +
			"\n\n\thelm s3 push --force " + localChartPath + " " + repositoryName +
			"\n\nError: plugin \"s3\" exited with error\n"
	requireCommandOutput(testSuite.T(), "", expectedErrorOutput, errors.Errorf("exit status 1"), output, errorOutput, err)
}

func (testSuite *EndToEndSuite) TestHelmS3PushContentType() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	contentType := defaultHelmChartContentType + "-test"
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath, "--content-type", contentType)

	object := getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	require.Equal(testSuite.T(), contentType, aws.StringValue(object.ContentType))
}

func (testSuite *EndToEndSuite) TestHelmS3PushDryRun() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath, "--dry-run")

	getNoAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
}

func (testSuite *EndToEndSuite) TestHelmS3PushForce() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath)

	object := getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	lastModified := aws.TimeValue(object.LastModified)

	time.Sleep(1 * time.Second) // Note: ensuring lastModified timestamp changes.

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath, "--force")

	object = getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	require.NotEqual(testSuite.T(), lastModified, aws.TimeValue(object.LastModified))
}

func (testSuite *EndToEndSuite) TestHelmS3PushIgnoreIfExists() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath)

	object := getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	lastModified := aws.TimeValue(object.LastModified)

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath, "--ignore-if-exists")

	object = getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)
	require.Equal(testSuite.T(), lastModified, aws.TimeValue(object.LastModified))
}

func (testSuite *EndToEndSuite) TestHelmS3PushForceAndIgnoreIfExists() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart

	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName

	output, errorOutput, err := tryPushHelmS3Chart(repositoryName, localChartPath, "--force", "--ignore-if-exists")
	expectedErrorOutput := "The --force and --ignore-if-exists flags are mutually exclusive and " +
		"cannot be specified together.\n" +
		"Error: plugin \"s3\" exited with error\n"
	requireCommandOutput(
		testSuite.T(),
		"",
		expectedErrorOutput,
		errors.Errorf("exit status 1"),
		output,
		errorOutput,
		err,
	)
}

func (testSuite *EndToEndSuite) TestHelmS3PushRelative() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	bucketRepositoryIndexPath := helmS3RepositoryFilePath("index.yaml")
	chart := exampleChart
	temporaryLocalIndexPath := testSuite.TemporaryPath(testName, "index.yaml")
	s3Client := testSuite.AWSS3Client()

	chartFileName := helmChartFileName(chart.Name, chart.Version)
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	repositoryName := bucketName
	temporaryLocalChartPath := testSuite.TemporaryPath(testName, helmChartFileName(chart.Name, chart.Version))

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath, "--relative")

	object := getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryIndexPath)

	saveAWSS3ObjectLocally( // nolint:gocritic // Note: intentional.
		testSuite.T(),
		object,
		temporaryLocalIndexPath,
		0o444,
	)

	chartVersion := helmIndexChartVersion(testSuite.T(), temporaryLocalIndexPath, chart.Name, chart.Version)
	require.Equal(testSuite.T(), []string{chartFileName}, chartVersion.URLs)

	fetchHelmChart(testSuite.T(), repositoryName, chart.Name, chart.Version, path.Dir(temporaryLocalChartPath))
}
