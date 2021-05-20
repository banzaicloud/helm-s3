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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/suite"
)

// EndToEndSuite collects the end to end tests in a test suite.
type EndToEndSuite struct {
	// Suite provides testify test suite functionality as base class.
	suite.Suite

	s3Client        *s3.S3
	testBucketNames map[string]string
}

// TestEndToEndSuite initiates the end to end test suite. Required for go test
// to run the testify test suite.
func TestEndToEndSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(EndToEndSuite))
}

// AfterTest executes test-specific behavior right after a test is run.
func (testSuite *EndToEndSuite) AfterTest(suiteName, testName string) {
	if testSuite == nil {
		return
	}

	bucketName := testSuite.AWSS3BucketName(testName)

	removeHelmRepository(testSuite.T(), bucketName)
	deleteAWSS3Bucket(testSuite.T(), testSuite.AWSS3Client(), bucketName)
	deleteDirectory(testSuite.T(), temporaryDirectoryPath(bucketName))
	testSuite.testBucketNames[testName] = ""
}

// AWSS3BucketName returns the name of the bucket corresponding of the specified
// test names's test.
func (testSuite *EndToEndSuite) AWSS3BucketName(testName string) string {
	if testSuite == nil ||
		testSuite.testBucketNames == nil {
		return ""
	}

	return testSuite.testBucketNames[testName]
}

// AWSS3Client returns the AWS S3 client object associated with the test suite.
func (testSuite *EndToEndSuite) AWSS3Client() *s3.S3 {
	if testSuite == nil {
		return nil
	}

	return testSuite.s3Client
}

// BeforeTest executes test-specific behavior right before a test is run.
func (testSuite *EndToEndSuite) BeforeTest(suiteName, testName string) {
	if testSuite == nil {
		return
	}

	bucketName := newUniqueBucketName(toLowerWordsFromCamelOrPascalCase(testName)...)
	repositoryURI := helmS3RepositoryURI(bucketName)

	testSuite.testBucketNames[testName] = bucketName
	createDirectory(testSuite.T(), temporaryDirectoryPath(bucketName), 0o755) // nolint:gocritic // Note: intentional.
	createAWSS3Bucket(testSuite.T(), testSuite.AWSS3Client(), bucketName)
	initializeHelmS3Repository(testSuite.T(), testSuite.s3Client, bucketName, repositoryURI)
	addHelmRepository(testSuite.T(), bucketName, repositoryURI)
}

// SetupSuite executes suite-independent behavior right before a suite is run.
func (testSuite *EndToEndSuite) SetupSuite() {
	if testSuite == nil {
		return
	}

	testSuite.testBucketNames = make(map[string]string)

	awsConfiguration := newAWSConfiguration()

	checkAWSEnvironment(testSuite.T(), awsConfiguration)
	initializeAWSEnvironment(testSuite.T(), awsConfiguration)

	testSuite.s3Client = s3.New(session.Must(session.NewSession(awsConfiguration)))

	_ = listAWSS3Buckets(testSuite.T(), testSuite.s3Client) // Note: API connection test.

	initializeHelmEnvironment(testSuite.T())
}

// TemporaryPath returns a path pointing into the test's temporary directory
// with a subpath based on the specified path elements.
func (testSuite *EndToEndSuite) TemporaryPath(testName string, pathElements ...string) string {
	if testSuite == nil {
		return ""
	}

	return temporaryDirectoryPath(append([]string{testSuite.AWSS3BucketName(testName)}, pathElements...)...)
}
