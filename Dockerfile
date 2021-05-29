ARG ALPINE_VERSION=3.13.5
ARG GO_VERSION=1.16.4

FROM golang:${GO_VERSION}-alpine as build

ARG ARCH=amd64
ARG HELM_PLUGIN_VERSION=local
ARG YQ_VERSION=v4.7.1

ENV YQ_BINARY="yq_linux_${ARCH}"

RUN apk add --no-cache \
    git \
    wget

RUN wget https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${YQ_BINARY} -O /usr/bin/yq && \
    chmod +x /usr/bin/yq

WORKDIR /workspace/helm-s3

COPY . .

# Note: Using argument version.
#
# Note: Not using install hooks in container context.
RUN yq eval --inplace ".version = \"${HELM_PLUGIN_VERSION}\"" plugin.yaml && \
    yq eval --inplace "del(.hooks)" plugin.yaml

RUN mkdir -p ./bin
RUN go build -ldflags "-X main.version=${HELM_PLUGIN_VERSION}" -o ./bin/helms3 ./cmd/helms3

FROM alpine:${ALPINE_VERSION} as helm

ARG HELM_VERSION=3.6.0

RUN apk add --update curl

# Install Helm binary for plugin installing.
RUN curl -L -s https://get.helm.sh/helm-v3.6.0-linux-amd64.tar.gz | tar -C /tmp -xvz && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm

FROM alpine:${ALPINE_VERSION}

COPY --from=build /workspace/helm-s3/LICENSE /root/.helm/cache/plugins/helm-s3/LICENSE
COPY --from=build /workspace/helm-s3/plugin.yaml /root/.helm/cache/plugins/helm-s3/plugin.yaml
COPY --from=build /workspace/helm-s3/bin/helms3 /root/.helm/cache/plugins/helm-s3/bin/helms3
COPY --from=helm /usr/local/bin/helm /usr/local/bin/helm

RUN mkdir -p /root/.helm/plugins \
    && helm plugin install /root/.helm/cache/plugins/helm-s3
