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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/banzaicloud/helm-s3/internal/awss3"
	"github.com/banzaicloud/helm-s3/internal/awsutil"
)

type proxyCmd struct {
	uri string
}

const indexYaml = "index.yaml"

func (act proxyCmd) Run(ctx context.Context) error {
	sess, err := awsutil.Session(
		awsutil.AssumeRoleTokenProvider(awsutil.StderrTokenProvider),
		awsutil.DynamicBucketRegion(act.uri),
	)
	if err != nil {
		return err
	}
	storage := awss3.New(sess)

	b, err := storage.FetchRaw(ctx, act.uri)
	if err != nil {
		if strings.HasSuffix(act.uri, indexYaml) && err == awss3.ErrObjectNotFound {
			return fmt.Errorf(
				"The index file does not exist by the path %s. "+
					"If you haven't initialized the repository yet, try running \"helm s3 init %s\"",
				act.uri,
				strings.TrimSuffix(strings.TrimSuffix(act.uri, indexYaml), "/"),
			)
		}
		return errors.WithMessage(err, fmt.Sprintf("fetch from s3 uri=%s", act.uri))
	}

	fmt.Print(string(b))
	return nil
}
