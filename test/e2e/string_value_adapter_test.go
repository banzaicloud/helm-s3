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
	"os/exec"
	"strings"
)

// stringValueAdapter describes an interface to obtain string values from any
// source.
type stringValueAdapter interface {
	// StringValue returns a string value obtained from a custom source.
	StringValue() (value string, isSet bool)
}

// dynamicStringValueAdapter implements a stringValueAdapter for dynamic
// sources.
type dynamicStringValueAdapter struct {
	valueFunction func() (string, bool)
}

// StringValue returns a string value obtained from a custom source.
func (adapter *dynamicStringValueAdapter) StringValue() (value string, isSet bool) {
	if adapter == nil {
		return "", false
	}

	return adapter.valueFunction()
}

// newDynamicStringValueAdapter returns a stringValueAdapter object from the
// specified function to use as a dynamic string value source.
func newDynamicStringValueAdapter(valueFunction func() (string, bool)) stringValueAdapter {
	return &dynamicStringValueAdapter{
		valueFunction: valueFunction,
	}
}

// newConfigurationFileStringValueAdapter returns a stringValueAdapter object
// from the specified profile with the provided key to use as a configuration
// file string value source.
func newConfigurationFileStringValueAdapter(key, profile string) stringValueAdapter {
	return newDynamicStringValueAdapter(
		func() (string, bool) {
			if profile == "" {
				profile = "default"
			}

			command := exec.Command("aws", "configure", "get", key, "--profile", profile)

			output, err := command.CombinedOutput()
			if err != nil {
				return "", false
			}

			value := strings.TrimSpace(string(output))
			if value == "" {
				return "", false
			}

			return value, true
		},
	)
}

// newEnvironmentStringValueAdapter returns a stringValueAdapter object from the
// specified environment key to use as an OS environment string value source.
func newEnvironmentStringValueAdapter(key string) stringValueAdapter {
	return newDynamicStringValueAdapter(
		func() (string, bool) {
			return os.LookupEnv(key)
		},
	)
}

// newFilePathStringValueAdapter returns a stringValueAdapter object from the
// specified string to use the file's path as a string value source.
func newFilePathStringValueAdapter(path string) stringValueAdapter {
	return newDynamicStringValueAdapter(
		func() (string, bool) {
			if _, err := os.Stat(path); err != nil {
				return "", false
			}

			return path, true
		},
	)
}

// newFirstStringValueAdapter returns a stringValueAdapter object from the
// specified adapters to use the first set value among the adapters as a string
// source.
func newFirstStringValueAdapter(adapters ...stringValueAdapter) stringValueAdapter {
	return newDynamicStringValueAdapter(
		func() (string, bool) {
			for _, adapter := range adapters {
				if value, isSet := adapter.StringValue(); isSet {
					return value, true
				}
			}

			return "", false
		},
	)
}

// newStaticStringValueAdapter returns a stringValueAdapter object from the
// specified string to use the static value as a string value source.
func newStaticStringValueAdapter(value string) stringValueAdapter {
	return newDynamicStringValueAdapter(
		func() (string, bool) {
			return value, true
		},
	)
}
