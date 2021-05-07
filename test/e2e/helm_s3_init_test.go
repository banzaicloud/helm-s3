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
)

func (testSuite *EndToEndSuite) TestHelmS3Init() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	bucketRepositoryIndexPath := helmS3RepositoryFilePath("index.yaml")
	s3Client := testSuite.AWSS3Client()

	repositoryName := bucketName
	repositoryURI := helmS3RepositoryURI(bucketName)

	// Note: undoing test setup.
	removeHelmRepository(testSuite.T(), repositoryName)
	deleteAWSS3Objects(testSuite.T(), s3Client, bucketName, bucketRepositoryIndexPath)

	initializeHelmS3Repository(testSuite.T(), s3Client, bucketName, repositoryURI)
}
