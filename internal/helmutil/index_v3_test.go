package helmutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestIndexV3_MarshalBinary(t *testing.T) {
	idx := IndexV3{
		index: &repo.IndexFile{
			APIVersion: "foo",
			Generated:  time.Date(2018, 01, 01, 0, 0, 0, 0, time.UTC),
		},
	}

	b, err := idx.MarshalBinary()
	require.NoError(t, err)

	expected := `apiVersion: foo
entries: null
generated: "2018-01-01T00:00:00Z"
`
	require.Equal(t, expected, string(b))
}

func TestIndexV3_UnmarshalBinary(t *testing.T) {
	input := []byte(`apiVersion: foo
entries: null
generated: 2018-01-01T00:00:00Z
`)

	idx := &IndexV3{}
	err := idx.UnmarshalBinary(input)
	require.NoError(t, err)

	require.Equal(t, "foo", idx.index.APIVersion)
	require.Equal(t, time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC), idx.index.Generated)
}

func TestIndexV3_AddOrReplace(t *testing.T) {
	t.Run("should add a new chart", func(t *testing.T) {
		i := newIndexV3()

		err := i.AddOrReplace(
			&chart.Metadata{
				Name:    "foo",
				Version: "0.1.0",
			},
			"foo-0.1.0.tgz",
			"http://example.com/charts",
			"sha256:1234567890",
		)
		require.NoError(t, err)

		require.Equal(t, "http://example.com/charts/foo-0.1.0.tgz", i.index.Entries["foo"][0].URLs[0])
	})

	t.Run("should add a new version of a chart", func(t *testing.T) {
		i := newIndexV3()

		err := i.AddOrReplace(
			&chart.Metadata{
				Name:    "foo",
				Version: "0.1.0",
			},
			"foo-0.1.0.tgz",
			"http://example.com/charts",
			"sha256:111",
		)
		require.NoError(t, err)

		err = i.AddOrReplace(
			&chart.Metadata{
				Name:    "foo",
				Version: "0.1.1",
			},
			"foo-0.1.1.tgz",
			"http://example.com/charts",
			"sha256:222",
		)
		require.NoError(t, err)

		i.SortEntries()

		require.Equal(t, "http://example.com/charts/foo-0.1.1.tgz", i.index.Entries["foo"][0].URLs[0])
		require.Equal(t, "sha256:222", i.index.Entries["foo"][0].Digest)
	})

	t.Run("should replace existing chart version", func(t *testing.T) {
		i := newIndexV3()

		err := i.AddOrReplace(
			&chart.Metadata{
				Name:    "foo",
				Version: "0.1.0",
			},
			"foo-0.1.0.tgz",
			"http://example.com/charts",
			"sha256:111",
		)
		require.NoError(t, err)

		err = i.AddOrReplace(
			&chart.Metadata{
				Name:    "foo",
				Version: "0.1.0",
			},
			"foo-0.1.0.tgz",
			"http://example.com/charts",
			"sha256:222",
		)
		require.NoError(t, err)

		require.Len(t, i.index.Entries, 1)

		require.Equal(t, "http://example.com/charts/foo-0.1.0.tgz", i.index.Entries["foo"][0].URLs[0])
		require.Equal(t, "sha256:222", i.index.Entries["foo"][0].Digest)
	})
}

func TestIndexV3WriteFile(t *testing.T) { // nolint:funlen // Note: table test.
	t.Parallel()

	type inputType struct {
		index *IndexV3
		dest  string
		mode  fs.FileMode
	}

	type outputType struct {
		err                error
		destinationContent string
	}

	temporaryDirectory, err := os.MkdirTemp(os.TempDir(), t.Name())
	require.NoError(t, err)

	testCases := []struct {
		caseDescription string
		expectedOutput  outputType
		input           inputType
	}{
		{
			caseDescription: "empty index, valid destination path -> success",
			expectedOutput: outputType{
				err: nil,
				destinationContent: `apiVersion: v1
entries: {}
generated: "0001-01-01T00:00:00Z"
`,
			},
			input: inputType{
				index: &IndexV3{
					index: &repo.IndexFile{ // nolint:exhaustivestruct // Note: minimal.
						APIVersion: repo.APIVersionV1,
						Generated:  time.Time{},
						Entries:    map[string]repo.ChartVersions{},
						PublicKeys: []string{},
					},
				},
				dest: filepath.Join(temporaryDirectory, "empty_index_valid_destination_path.yaml"),
				mode: fs.ModePerm,
			},
		},
		{
			caseDescription: "not empty index, single chart, single version, valid destination path -> success",
			expectedOutput: outputType{
				err: nil,
				destinationContent: `apiVersion: v1
entries:
  exampleChart:
  - created: "0001-01-01T00:00:00Z"
    digest: sha256:0123456789
    name: example
    urls:
    - https://example.com/charts
    version: 0.1.0
generated: "0001-01-01T00:00:00Z"
`,
			},
			input: inputType{
				index: &IndexV3{
					index: &repo.IndexFile{ // nolint:exhaustivestruct // Note: minimal.
						APIVersion: repo.APIVersionV1,
						Generated:  time.Time{},
						Entries: map[string]repo.ChartVersions{
							"exampleChart": []*repo.ChartVersion{
								{
									URLs: []string{"https://example.com/charts"},
									Metadata: &chart.Metadata{ // nolint:exhaustivestruct // Note: minimal.
										Name:    "example",
										Version: "0.1.0",
									},
									Digest:  "sha256:0123456789",
									Created: time.Time{},
								},
							},
						},
						PublicKeys: []string{},
					},
				},
				dest: filepath.Join(temporaryDirectory, "not_empty_index_single_chart_single_version_valid_destination_path.yaml"),
				mode: fs.ModePerm,
			},
		},
		{
			caseDescription: "not empty index, multiple charts, multiple versions, valid destination path -> success",
			expectedOutput: outputType{
				err: nil,
				destinationContent: `apiVersion: v1
entries:
  exampleChart:
  - created: "0001-01-01T00:00:00Z"
    digest: sha256:0123456789
    name: example
    urls:
    - https://example.com/charts
    version: 0.1.0
  multipleChart:
  - created: "0001-01-01T00:00:00Z"
    digest: sha256:1
    name: multipleExample
    urls:
    - https://example.com/charts
    version: 1.0.0
  - created: "0001-01-01T00:00:00Z"
    digest: sha256:2
    name: multipleExample
    urls:
    - https://example.com/charts
    version: 2.0.0
generated: "0001-01-01T00:00:00Z"
`,
			},
			input: inputType{
				index: &IndexV3{
					index: &repo.IndexFile{ // nolint:exhaustivestruct // Note: minimal.
						APIVersion: repo.APIVersionV1,
						Generated:  time.Time{},
						Entries: map[string]repo.ChartVersions{
							"exampleChart": []*repo.ChartVersion{
								{
									URLs: []string{"https://example.com/charts"},
									Metadata: &chart.Metadata{ // nolint:exhaustivestruct // Note: minimal.
										Name:    "example",
										Version: "0.1.0",
									},
									Digest:  "sha256:0123456789",
									Created: time.Time{},
								},
							},
							"multipleChart": []*repo.ChartVersion{
								{
									URLs: []string{"https://example.com/charts"},
									Metadata: &chart.Metadata{ // nolint:exhaustivestruct // Note: minimal.
										Name:    "multipleExample",
										Version: "1.0.0",
									},
									Digest:  "sha256:1",
									Created: time.Time{},
								},
								{
									URLs: []string{"https://example.com/charts"},
									Metadata: &chart.Metadata{ // nolint:exhaustivestruct // Note: minimal.
										Name:    "multipleExample",
										Version: "2.0.0",
									},
									Digest:  "sha256:2",
									Created: time.Time{},
								},
							},
						},
						PublicKeys: []string{},
					},
				},
				dest: filepath.Join(
					temporaryDirectory,
					"not_empty_index_multiple_charts_multiple_versions_valid_destination_path.yaml",
				),
				mode: fs.ModePerm,
			},
		},
		{
			caseDescription: "nil index -> error",
			expectedOutput: outputType{
				err:                ErrorNilIndex,
				destinationContent: "",
			},
			input: inputType{
				index: nil,
				dest:  filepath.Join(temporaryDirectory, "nil_index.yaml"),
				mode:  fs.ModePerm,
			},
		},
		{
			caseDescription: "empty destination path -> error",
			expectedOutput: outputType{
				err:                ErrorMissingDestination,
				destinationContent: "",
			},
			input: inputType{
				index: &IndexV3{
					index: &repo.IndexFile{}, // nolint:exhaustivestruct // Note: minimal.
				},
				dest: "",
				mode: fs.ModePerm,
			},
		},
		{
			caseDescription: "write error -> error",
			expectedOutput: outputType{
				err: errors.Errorf(
					"writing index file failed" +
						": error marshaling into JSON" +
						": json: unsupported type: map[bool]string",
				),
				destinationContent: "",
			},
			input: inputType{
				index: &IndexV3{
					index: &repo.IndexFile{ // nolint:exhaustivestruct // Note: minimal.
						ServerInfo: map[string]interface{}{
							"invalid": map[bool]string{
								false: "false",
								true:  "true",
							},
						},
					},
				},
				dest: filepath.Join(temporaryDirectory, "write_error_index.yaml"),
				mode: fs.ModePerm,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.caseDescription, func(t *testing.T) {
			t.Parallel()

			actualError := testCase.input.index.WriteFile(testCase.input.dest, testCase.input.mode)

			if testCase.expectedOutput.err == nil {
				require.NoError(t, actualError)
				require.FileExists(t, testCase.input.dest)

				fileInfo, err := os.Stat(testCase.input.dest)
				require.NoError(t, err, "os.Stat() failed on destination file")
				require.Equal(t, testCase.input.mode, fileInfo.Mode())

				content, err := os.ReadFile(testCase.input.dest)
				require.NoError(t, err, "os.ReadFile() failed on destination path")
				require.Equal(t, testCase.expectedOutput.destinationContent, string(content))
			} else {
				require.EqualError(t, actualError, testCase.expectedOutput.err.Error(), "")
				require.NoFileExists(t, testCase.input.dest)
			}
		})
	}
}
