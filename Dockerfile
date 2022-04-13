FROM golang:1.18-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY *.go .

RUN go build -o /app/imagehost
RUN rm go.mod go.sum *.go

FROM alpine:3.15

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY example_docker.toml /app/config.toml
COPY --from=builder /app/imagehost /app/imagehost
COPY template/ template/
COPY public/ public/

ENTRYPOINT [ "/app/imagehost", "-c", "/app/config.toml" ]