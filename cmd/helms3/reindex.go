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
	"log"

	"github.com/pkg/errors"

	"github.com/banzaicloud/helm-s3/internal/awss3"
	"github.com/banzaicloud/helm-s3/internal/awsutil"
	"github.com/banzaicloud/helm-s3/internal/helmutil"
)

type reindexAction struct {
	repoName string
	acl      string
	relative bool
}

func (act reindexAction) Run(ctx context.Context) error {
	repoEntry, err := helmutil.LookupRepoEntry(act.repoName)
	if err != nil {
		return err
	}

	sess, err := awsutil.Session(awsutil.DynamicBucketRegion(repoEntry.URL()))
	if err != nil {
		return err
	}
	storage := awss3.New(sess)

	items, errs := storage.Traverse(ctx, repoEntry.URL())

	builtIndex := make(chan helmutil.Index, 1)
	go func() {
		idx := helmutil.NewIndex()
		for item := range items {
			baseURL := repoEntry.URL()
			if act.relative {
				baseURL = ""
			}
			if err := idx.Add(item.Meta.Value(), item.Filename, baseURL, item.Hash); err != nil {
				log.Printf("[ERROR] failed to add chart to the index: %s", err)
			}
		}
		idx.SortEntries()

		builtIndex <- idx
	}()

	for err = range errs {
		return errors.Wrap(err, "traverse the chart repository")
	}

	idx := <-builtIndex

	r, err := idx.Reader()
	if err != nil {
		return errors.Wrap(err, "get index reader")
	}

	if err := storage.PutIndex(ctx, repoEntry.URL(), act.acl, r); err != nil {
		return errors.Wrap(err, "upload index to the repository")
	}

	if err := idx.WriteFile(repoEntry.CacheFile(), 0644); err != nil {
		return errors.WithMessage(err, "update local index")
	}

	return nil
}
