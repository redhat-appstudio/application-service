FROM registry.access.redhat.com/ubi9/go-toolset:latest

WORKDIR /app

COPY  . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -o cdq-analysis main.go

USER 1001

ENTRYPOINT ["/app/entrypoint.sh"]

