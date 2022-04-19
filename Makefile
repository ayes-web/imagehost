build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./bin/imagehost
	tar czf ./bin/build.tar.gz ./bin/imagehost template/ public/

clean:
	rm -rf ./bin

docker:
	docker build -t ayay2021/imagehost .

docker-push:
	docker push ayay2021/imagehost:latest