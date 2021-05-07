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

	"github.com/stretchr/testify/require"
)

func (testSuite *EndToEndSuite) TestHelmS3Delete() {
	testName := path.Base(testSuite.T().Name())

	bucketName := testSuite.AWSS3BucketName(testName)
	chart := exampleChart
	localChartPath := testChartPath(testSuite.T(), chart.Name, chart.Version)
	s3Client := testSuite.AWSS3Client()

	bucketRepositoryChartPath := helmS3RepositoryChartPath(chart.Name, chart.Version)
	repositoryName := bucketName

	pushHelmS3Chart(testSuite.T(), repositoryName, localChartPath)

	_ = getAWSS3Object(testSuite.T(), s3Client, bucketName, bucketRepositoryChartPath)

	require.Contains(testSuite.T(), searchHelmCharts(testSuite.T(), repositoryName, chart.Name), chart)

	deleteHelmS3Chart(testSuite.T(), repositoryName, chart.Name, chart.Version)

	getNoAWSS3Object(testSuite.T(), s3Client, repositoryName, bucketRepositoryChartPath)

	require.Empty(testSuite.T(), searchHelmCharts(testSuite.T(), repositoryName, chart.Name))
}
