FROM golang:1.17-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY auto_deletion.go .
COPY main.go .
COPY api.go .
COPY admin.go .

RUN go build -o /app/imagehost
RUN rm go.mod go.sum main.go auto_deletion.go api.go admin.go

FROM alpine:3.15

VOLUME [ "/app/data" ]
EXPOSE 80
WORKDIR /app

COPY example_docker.toml /app/config.toml
COPY --from=builder /app/imagehost /app/imagehost
COPY public/ public/
COPY template/ template/

ENTRYPOINT [ "/app/imagehost", "-c", "/app/config.toml" ]