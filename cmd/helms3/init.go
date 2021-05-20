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

	"github.com/pkg/errors"

	"github.com/banzaicloud/helm-s3/internal/awss3"
	"github.com/banzaicloud/helm-s3/internal/awsutil"
	"github.com/banzaicloud/helm-s3/internal/helmutil"
)

type initAction struct {
	uri string
	acl string
}

func (act initAction) Run(ctx context.Context) error {
	r, err := helmutil.NewIndex().Reader()
	if err != nil {
		return errors.WithMessage(err, "get index reader")
	}

	sess, err := awsutil.Session(awsutil.DynamicBucketRegion(act.uri))
	if err != nil {
		return err
	}
	storage := awss3.New(sess)

	if err := storage.PutIndex(ctx, act.uri, act.acl, r); err != nil {
		return errors.WithMessage(err, "upload index to s3")
	}

	// TODO:
	// do we need to automatically do `helm repo add <name> <uri>`,
	// like we are doing `helm repo update` when we push a chart
	// with this plugin?

	return nil
}
