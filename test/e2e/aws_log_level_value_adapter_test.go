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
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
)

// awsLogLevelValueAdapter describes an interface to obtain AWS log level values
// from any source.
type awsLogLevelValueAdapter interface {
	// AWSLogLevelValue returns an AWS log level value obtained from a custom
	// source.
	AWSLogLevelValue() (value aws.LogLevelType, isSet bool)
}

// dynamicAWSLogLevelValueAdapter implements a stringValueAdapter for dynamic
// sources.
type dynamicAWSLogLevelValueAdapter struct {
	valueFunction func() (aws.LogLevelType, bool)
}

// AWSLogLevel returns an AWS log level value obtained from a custom source.
func (adapter *dynamicAWSLogLevelValueAdapter) AWSLogLevelValue() (value aws.LogLevelType, isSet bool) {
	if adapter == nil {
		return aws.LogOff, false
	}

	return adapter.valueFunction()
}

// newDynamicAWSLogLevelValueAdapter returns an awsLogLevelValueAdapter object
// from the specified function to use as a dynamic AWS log level value source.
func newDynamicAWSLogLevelValueAdapter(valueFunction func() (aws.LogLevelType, bool)) awsLogLevelValueAdapter {
	return &dynamicAWSLogLevelValueAdapter{
		valueFunction: valueFunction,
	}
}

// newEnvironmentAWSLogLevelValueAdapter returns an awsLogLevelValueAdapter
// object from the specified environment key to use as an OS environment
// AWSLogLevel value source.
func newEnvironmentAWSLogLevelValueAdapter(key string) awsLogLevelValueAdapter {
	return newDynamicAWSLogLevelValueAdapter(
		func() (aws.LogLevelType, bool) {
			awsLogLevelString, isSet := os.LookupEnv(key)
			if !isSet {
				return aws.LogOff, false
			}

			awsLogLevelUint64, err := strconv.ParseUint(awsLogLevelString, 10, 64)
			if err != nil {
				return aws.LogOff, false
			}

			return aws.LogLevelType(awsLogLevelUint64), true
		},
	)
}

// newFirstAWSLogLevelValueAdapter returns an awsLogLevelValueAdapter object
// from the specified adapters to use the first set value among the adapters as
// a AWSLogLevel source.
func newFirstAWSLogLevelValueAdapter(adapters ...awsLogLevelValueAdapter) awsLogLevelValueAdapter {
	return newDynamicAWSLogLevelValueAdapter(
		func() (aws.LogLevelType, bool) {
			for _, adapter := range adapters {
				if value, isSet := adapter.AWSLogLevelValue(); isSet {
					return value, true
				}
			}

			return aws.LogOff, false
		},
	)
}

// newStaticAWSLogLevelValueAdapter returns an awsLogLevelValueAdapter object
// from the specified AWSLogLevel to use the static value as a AWSLogLevel value
// source.
func newStaticAWSLogLevelValueAdapter(value aws.LogLevelType) awsLogLevelValueAdapter {
	return newDynamicAWSLogLevelValueAdapter(
		func() (aws.LogLevelType, bool) {
			return value, true
		},
	)
}
