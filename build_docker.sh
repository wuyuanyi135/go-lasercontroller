#!/usr/bin/env bash

docker build -f Dockerfile.amd64 --tag wuyuanyi/mvcamctrl:latest-amd64 .
docker build -f Dockerfile.arm64 --tag wuyuanyi/mvcamctrl:latest-arm64v8 .