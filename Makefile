build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./bin/imagehost
	tar czf ./bin/build.tar.gz ./bin/imagehost

clean:
	rm -rf ./bin
	go clean

docker:
	docker build -t ayay2021/imagehost .

docker-push:
	docker push ayay2021/imagehost:latest