build:
	CGO_ENABLED=0 go build -ldflags "-s -w"

clean:
	go clean

docker:
	docker build -t ayay2021/imagehost .

docker-push:
	docker push ayay2021/imagehost:latest