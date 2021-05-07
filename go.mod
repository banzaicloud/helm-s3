module github.com/banzaicloud/helm-s3

go 1.15

// See: https://github.com/helm/helm/issues/9354
replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
)

require (
	emperror.dev/errors v0.8.0
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15 // indirect
	github.com/aws/aws-sdk-go v1.38.35
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/google/uuid v1.1.2
	github.com/mitchellh/mapstructure v1.4.1
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/multierr v1.7.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	helm.sh/helm/v3 v3.5.4
	k8s.io/helm v2.17.0+incompatible
	sigs.k8s.io/yaml v1.2.0
)
