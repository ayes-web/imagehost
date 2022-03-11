#!/bin/env bash
echo "building"

./clean.sh
mkdir ./bin
CGO_ENABLED=0 go build -ldflags "-s -w" -o ./bin/imagehost
# upx ./bin/imagehost
tar czf ./bin/build.tar.gz ./bin/imagehost template/ public/

echo "done"