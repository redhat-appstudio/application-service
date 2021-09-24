
.PHONY: build
build: fmt vet
	CGO_ENABLED=0 go build -o .

.PHONY: test
test: fmt vet
	go test -v -race -count=1 -coverprofile=coverage.out `go list ./...`

.PHONY: gosec
gosec: 
	# Run this command, to install gosec, if not installed:
	# go get -u github.com/securego/gosec/v2/cmd/gosec
	gosec ./...

.PHONY: lint
lint:
	golangci-lint --version
	GOMAXPROCS=2 golangci-lint run --fix --verbose --timeout 300s

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...
