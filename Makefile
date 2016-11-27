.PHONY: build install autolint clean test

THIS_FILE := $(lastword $(MAKEFILE_LIST))
ROOT_DIR = $(shell pwd)
GIT_REF_SHA=$(shell git rev-parse --short HEAD)
CMDS=$(shell ls cmd)
GIT_REF_LABEL=$(shell git symbolic-ref -q --short HEAD || git describe --tags --exact-match)
GIT_DIRTY=$(shell git diff-index --quiet HEAD; echo $$? | sed 's/1/-dirty/' | sed 's/0//')
BUILD_TIME=$(shell date +%FT%T%z)
PKG_PATH=github.com/codeactual/ec2metaproxy
LDFLAGS=-ldflags "-X ${PKG_PATH}/version.SCM=${GIT_REF_SHA}-${GIT_REF_LABEL}${GIT_DIRTY} -X ${PKG_PATH}/version.BuildTime=${BUILD_TIME}"

precheck:
ifneq ($(CGO_ENABLED),0)
	$(error precheck: CGO_ENABLED is not 0 in environment)
endif

build:
	@mkdir -p build
	@go build ${LDFLAGS} -v -o build/ec2metaproxy

install:
	@go install ${LDFLAGS} .

autolint:
	@reflex -c reflex.conf

clean:
	@go clean -i

test:
	@CGO_ENABLED=1 go test -v -race github.com/codeactual/ec2metaproxy/proxy

-include precheck
