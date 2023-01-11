FROM registry.access.redhat.com/ubi9/go-toolset:latest

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o ./main
