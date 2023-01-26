FROM registry.access.redhat.com/ubi9/go-toolset:latest

#USER root
WORKDIR /app

#COPY go.mod .
#COPY go.sum .
COPY  . .
#RUN chmod 755 go.mod \
# && chmod 755 go.sum
#RUN go mod tidy

RUN go mod download

RUN go build -o ./main

