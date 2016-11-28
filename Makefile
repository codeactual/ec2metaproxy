.PHONY: build install autolint clean test builder runner

THIS_FILE := $(lastword $(MAKEFILE_LIST))
ROOT_DIR = $(shell pwd)
GIT_REF_SHA=$(shell git rev-parse --short HEAD)
CMDS=$(shell ls cmd)
GIT_REF_LABEL=$(shell git symbolic-ref -q --short HEAD || git describe --tags --exact-match)
GIT_DIRTY=$(shell git diff-index --quiet HEAD; echo $$? | sed 's/1/-dirty/' | sed 's/0//')
BUILD_TIME=$(shell date +%FT%T%z)
PKG_PATH=github.com/codeactual/ec2metaproxy
LDFLAGS=-ldflags "-X ${PKG_PATH}/version.SCM=${GIT_REF_SHA}-${GIT_REF_LABEL}${GIT_DIRTY} -X ${PKG_PATH}/version.BuildTime=${BUILD_TIME}"

build:
	@mkdir -p build
	@CGO_ENABLED=0 go build ${LDFLAGS} -v -o build/ec2metaproxy
	ls -la build

install:
	@CGO_ENABLED=0 go install ${LDFLAGS} .

autolint:
	@# `make install` as hack to update so linters use fresh object files.
	@# Ensure cgo is enabled to avoid file permission issues under /usr/local/go.
	@CGO_ENABLED=1 go install ${LDFLAGS} .
	@gometalinter --vendor --concurrency=2 --deadline=60s --disable=aligncheck $(DIR) | head -n 15

watch:
	@reflex -c reflex.conf

clean:
	@go clean -i

test:
	@CGO_ENABLED=1 go test -v -race github.com/codeactual/ec2metaproxy/proxy

builder:
	@docker build --rm -t ec2metaproxy:builder --build-arg GIT_REF=$(GIT_REF) --no-cache -f Dockerfile.build .
	@docker images | grep ec2metaproxy

runner:
	@docker run --rm ec2metaproxy:builder  | docker build --rm -t ec2metaproxy:$(TAG) --no-cache -f Dockerfile.run -
	@docker images | grep ec2metaproxy

docker: builder runner

docker_latest:
	@GIT_REF=HEAD TAG=latest $(MAKE) -f $(THIS_FILE) docker
