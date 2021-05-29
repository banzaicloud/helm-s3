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

package helmutil

import "emperror.dev/errors"

var (
	// ErrorMissingDestination is the error returned when an empty destination
	// file path is received for a required path.
	ErrorMissingDestination = errors.Errorf("required destination path is missing")

	// ErrorNilIndex is the error returned when not nil index is expected, but
	// nil index is received.
	ErrorNilIndex = errors.Errorf("index is nil")
)
