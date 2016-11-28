# Binary only

    make build

# Docker

## Quick

    make docker_latest

This will create an `ec2metaproxy:latest` image from this repository's `HEAD`.

## Customized

    GIT_REF=efd25a2 TAG=1.0 make docker

- `GIT_REF`: a commit from this repository
- `TAG`: a value of `1.0` will create the Docker image `ec2metaproxy:1.0`

> The above command will follow an [approach](https://joeshaw.org/smaller-docker-containers-for-go-apps/) creates two images but allows us to isolate the entire process inside containers.

If we look inside the `Makefile`, the process consists of two steps.

First, we make "builder" image, `ec2metaproxy:builder` whose containers will just emit a `tar` to `STDOUT`. The `tar` will contain two files: `Dockerfile.run` and the `ec2metaproxy` binary.

It emits `tar` to take advantage of the `docker build` feature which supports passing the `Dockerfile` and context over `STDIN`.

    GIT_REF=efd25a2 make builder

Second, we make the image to actually run `ec2metaproxy`.

    TAG=1.0 make runner

