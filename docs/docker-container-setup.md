By default, a container receives the default permissions configured by the metadata
proxy. However, some containers require more or less permissions. For these cases, the
container can provide a custom IAM role, a custom IAM policy, or both.

The role and/or policy are configured in container labels. The metadata
proxy daemon uses the docker API to get the configured role and policy for a container.
The labels can only be set when the container is created and can not be
modified while the container is running (as of Docker Engine `1.12.3`, see [related feature request](https://github.com/docker/docker/issues/21721)).

# Container Role

A container can specify a specific role to use by setting the `ec2metaproxy.RoleAlias` label
on the image or the container.  The metadata proxy will return credentials for the given role
when requested.

Example:

```bash
docker run --label "ec2metaproxy.RoleAlias=db" ...
```

The `aliasToARN` section of the JSON config file might look like:

    "aliasToARN": {
      "default": "arn:aws:iam::000000000000:role/ProxyDefault",
      "db": "arn:aws:iam::000000000000:role/MysqlSlave"
    }

Note that the host machine’s instance profile must have permission to assume the given role.
If not, the container will receive an error when requesting the credentials.

# Container Policy

A container can specify a custom IAM policy by setting the `ec2metaproxy.Policy` label
the image or the container. The resulting container permissions will be the intersection
of the custom policy and the default container role or the role specified by the container's
`ec2metaproxy.RoleAlias` label.

Example:

```bash
docker run --label 'ec2metaproxy.Policy={"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:DescribeInstances"],"Resource":["*"]}]}' ...
```
