# ec2metaproxy [![GoDoc](https://godoc.org/github.com/codeactual/ec2metaproxy/proxy?status.svg)](https://godoc.org/github.com/codeactual/ec2metaproxy/proxy) [![Go Report Card](https://goreportcard.com/badge/github.com/codeactual/ec2metaproxy)](https://goreportcard.com/report/github.com/codeactual/ec2metaproxy) [![Build Status](https://travis-ci.org/codeactual/ec2metaproxy.png)](https://travis-ci.org/codeactual/ec2metaproxy)

A service that runs on an EC2 instance that proxies the EC2 instance metadata service
for Docker containers. The proxy overrides metadata endpoints for individual
containers via `iptables`.

At this point, the only endpoint overridden is the security credentials. This allows
for different containers to have different IAM permissions and not just use the permissions
provided by the instance profile. However, this same technique could be used to override
any other endpoints where appropriate.

The proxy works by mapping the metadata source request IP to the container using the container
platform specific API. The container's metadata contains information about what IAM permissions
to use. Therefore, the proxy does not work for containers that do not use the container
network bridge (for example, containers using "host" networking).

# Fork Notes

Goals of this https://github.com/dump247/ec2metaproxy fork:

- Adopt a different way to map containers to the roles they assume:
  - Store the mapping (and other settings) in a JSON config file.
  - Map free-form aliases to role ARNs in the config file.
  - Use docker's built-in [labels](https://docs.docker.com/search/?q=container+labels) to store the aliases.
- Reduce dependencies in favor of `log`, `flag`, and the official docker client package.
- Refactor most of the project into its `proxy` package where `main()` is just a client.
- Improve test coverage.
- Reduce `panic` use to only `rand.Read` errors on `proxy` package `init()`.
- Remove `flynn` support since I cannot regularly test/maintain correctness.
- Add optional HTTP request/response logs.

# Setup

## Host

The host EC2 instance must have firewall settings that redirect any EC2 metadata connections
from containers to the metadata proxy. The proxy will then process the request and
may forward the request to the real metadata service.

The instance profile of the host EC2 instance must also have permission to assume the IAM roles
for the containers.

See:

- [Host Setup](docs/host-setup.md)

## Containers

Containers do not require any changes or modifications to utilize the metadata proxy. By
default, they will receive the default permissions configured by the proxy. Alternatively,
a container can be configured to use a separate IAM role or provide an IAM policy.

See:

- [Docker Container Setup](docs/docker-container-setup.md)

# Walkthrough

## Build

Options:

1. Binary only from current git clone: `make build`
1. Docker image `ec2metaproxy:latest` from `HEAD`: `make docker_latest`
1. Docker image with [customization](docs/build.md): `GIT_REF=efd25a2 TAG=1.0 make docker`
1. Docker image from current git clone: `TAG=testing make docker_local`

## Create a JSON config file

Example that specifies all settings:

    {
      "defaultAlias": "default",
      "aliasToARN": {
        "default": "arn:aws:iam::000000000000:role/ProxyDefault",
        "db": "arn:aws:iam::000000000000:role/MysqlSlave"
      },
      "dockerHost": "unix:///var/run/custom-docker.sock",
      "listen": ":18000",
      "verbose": true
    }

Required settings:

- `listen`
- `aliasToARN`
- `defaultAlias`

## Forward traffic from containers to the proxy

     ./scripts/setup-firewall.sh --container-iface docker0 --proxy-port 18000

 See `--help` for additional flags.

## Run

Options:

1. Binary only: `ec2metaproxy -c config.json`
1. Docker: `./scripts/run-docker.sh --config config.json` (see `--help` for additional flags)

# Tests

## Run

    make test

# Dependencies

- https://github.com/aws/aws-sdk-go (Apache 2.0, vendored)
- https://github.com/docker/docker (Apache 2.0, vendored)
- https://github.com/pkg/errors (BSD-2-Clause, vendored)
- `request_id.go` middleware from https://github.com/zenazn/goji (MIT, embedded)

# License

The MIT License (MIT)
Copyright (c) 2014 Cory Thomas

See [LICENSE](LICENSE)
