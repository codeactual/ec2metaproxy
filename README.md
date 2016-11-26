A service that runs on an EC2 instance that proxies the EC2 instance metadata service
for linux containers. The proxy overrides metadata endpoints for individual
containers.

The following container platforms are supported:

- [docker](https://www.docker.com)

At this point, the only endpoint overridden is the security credentials. This allows
for different containers to have different IAM permissions and not just use the permissions
provided by the instance profile. However, this same technique could be used to override
any other endpoints where appropriate.

The proxy works by mapping the metadata source request IP to the container using the container
platform specific API. The container's metadata contains information about what IAM permissions
to use. Therefore, the proxy does not work for containers that do not use the container
network bridge (for example, containers using "host" networking).

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

## Configuration File

Example that specifies all options:

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

Select it with: `ec2metaproxy -c /path/to/config.json`

# Fork Notes

Goals of this fork:

- Adopt a different way to map containers to the roles they assume:
  - Use a JSON config file instead of environment variable injection at `docker run`.
  - Map custom aliases to role ARNs in the config file.
  - Use docker's built-in object labels to store the aliases.
- Reduce dependencies in favor of `log`, `flag`, and the official docker client package.
- Refactor most of the project into its `proxy` package where `main()` is just a client.
- Reduce `panic` use to only `rand.Read` errors on `proxy` package `init()`.
- Remove `flynn` support.

# License

The MIT License (MIT)
Copyright (c) 2014 Cory Thomas

See [LICENSE](LICENSE)
