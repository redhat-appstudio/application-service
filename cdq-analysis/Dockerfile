FROM registry.access.redhat.com/ubi9/go-toolset:latest

WORKDIR /app

COPY  . .

RUN go mod download

RUN go build -o ./main

