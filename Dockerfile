# Build the manager binary
FROM golang:1.20 as builder
ARG TARGETOS
ARG TARGETARCH
ARG KUSTOMIZE_VERSION="v5.3.0"

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Add a go build cache. The persistent cache helps speed up build steps,
# especially steps that involve installing packages using a package manager.
ENV GOCACHE=/root/.cache/go-build
# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN --mount=type=cache,target="/root/.cache/go-build" CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o manager cmd/main.go

# Download kustomize binary
RUN curl --silent --location --remote-name \
    "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" && \
    tar -C /tmp -xf kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz && \
    install -m 0755 /tmp/kustomize /usr/local/bin/kustomize

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
USER 65532:65532

ENTRYPOINT ["/manager"]
