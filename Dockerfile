FROM golang:1.18-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY admin.go .
COPY api.go .
COPY auto_deletion.go .
COPY handlers.go .
COPY main.go .

RUN go build -o /app/imagehost
RUN rm go.mod go.sum admin.go api.go auto_deletion.go handlers.go main.go

FROM alpine:3.15

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY example_docker.toml /app/config.toml
COPY --from=builder /app/imagehost /app/imagehost
COPY template/ template/
COPY public/ public/

ENTRYPOINT [ "/app/imagehost", "-c", "/app/config.toml" ]