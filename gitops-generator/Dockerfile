# Build the gitops generator binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY pkg pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o gitops-generator main.go


FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-751
RUN microdnf update --setopt=install_weak_deps=0 -y && microdnf install git

# Set up the non-root workspace and copy over the gitops generator binary and entrypoint script
WORKDIR /workspace
COPY --from=builder /workspace/gitops-generator .

COPY entrypoint.sh .
RUN chgrp -R 0 /workspace && chmod -R g=u /workspace

USER 1001

ENTRYPOINT ["/workspace/entrypoint.sh"]