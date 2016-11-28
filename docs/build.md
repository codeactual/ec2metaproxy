# Binary only

    make build

# Docker

## Quick

    make docker_latest

This will create an `ec2metaproxy:latest` image from this repository's `HEAD`.

## Customized

    GIT_REF=HEAD TAG=latest make docker

- `GIT_REF`: a commit from this repository
- `TAG`: a value of `latest` will create the Docker image `ec2metaproxy:latest`

> The above command will follow an [approach](https://joeshaw.org/smaller-docker-containers-for-go-apps/) creates two images but allows us to isolate the entire process inside containers.

If we look inside the `Makefile`, the process consists of two steps.

First, we make "builder" image, `ec2metaproxy:builder` whose containers will just emit a `tar` to `STDOUT`. The `tar` will contain two files: `Dockerfile.run` and the `ec2metaproxy` binary.

It emits `tar` to take advantage of the `docker build` feature which supports passing the `Dockerfile` and context over `STDIN`.

    GIT_REF=HEAD make builder
    docker images | grep "ec2metaproxy.*builder"

Second, we make the image to actually run `ec2metaproxy`.

    TAG=latest make runner
    docker images | grep "ec2metaproxy.*latest"

