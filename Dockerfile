# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
# ToDo: Uncomment once API added
COPY api/ api/
COPY controllers/ controllers/
COPY pkg pkg/
COPY gitops gitops/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -o manager main.go


FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6-751
RUN microdnf update --setopt=install_weak_deps=0 -y && microdnf install git

ARG ENABLE_WEBHOOKS=true
ENV ENABLE_WEBHOOKS=${ENABLE_WEBHOOKS}

# Set the Git config for the AppData bot
WORKDIR /
COPY --from=builder /workspace/manager .

COPY appdata.gitconfig /.gitconfig
RUN chgrp -R 0 /.gitconfig && chmod -R g=u /.gitconfig

USER 1001

ENTRYPOINT ["/manager"]
