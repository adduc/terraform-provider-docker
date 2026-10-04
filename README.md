# Terraform Provider: Docker (read-only)

A Terraform and OpenTofu provider for reading files, logs, and server
information from Docker containers.

It only reads: it has no resources and never changes a container. Use it
to feed values out of running containers (a generated config file, a
token written at startup, a log line) into the rest of your
configuration.

## Data sources

| Data source | Description |
|---|---|
| [`docker_file`](docs/data-sources/file.md) | A single file's stats and contents |
| [`docker_files`](docs/data-sources/files.md) | All files under a path, as a map |
| [`docker_logs`](docs/data-sources/logs.md) | A container's logs, one entry per line, with the stream and timestamp |
| [`docker_server_version`](docs/data-sources/server_version.md) | The Docker server's version, platform, and components |

Full documentation is in [`docs/`](docs/index.md) and on the
[Terraform Registry](https://registry.terraform.io/providers/adduc/docker/latest/docs).

## Usage

```terraform
terraform {
  required_providers {
    docker = {
      source = "adduc/docker"
      # OpenTofu: source = "registry.terraform.io/adduc/docker"
    }
  }
}

provider "docker" {}

data "docker_file" "repositories" {
  container = "alpine"
  path      = "/etc/apk/repositories"
}

output "repositories" {
  value     = base64decode(data.docker_file.repositories.file.content_base64)
  sensitive = true
}
```

File contents are returned base64-encoded so binary files survive intact,
and are marked sensitive since they often hold secrets.

### Connecting to Docker

By default the provider connects to the local Docker daemon. Like the
Docker CLI, it honors `DOCKER_HOST`, `DOCKER_CERT_PATH`,
`DOCKER_TLS_VERIFY`, and `DOCKER_API_VERSION`. To set the connection
explicitly:

```terraform
provider "docker" {
  host      = "tcp://docker.example.com:2376" # or unix://, ssh://user@host
  cert_path = "/home/me/.docker"              # ca.pem, cert.pem, key.pem for TLS
  timeout   = 30                              # seconds
}
```

## Development

Requires [Go](https://go.dev/) (see `go.mod` for the version) and Docker.

```sh
make build     # build the provider
make install   # install it into $GOPATH/bin
make test      # unit tests
make testacc   # acceptance tests against the Docker daemon from DOCKER_HOST
make generate  # format examples and regenerate docs/ (needs Terraform)
```

To run the acceptance tests with OpenTofu:

```sh
TF_ACC_TERRAFORM_PATH=$(which tofu) TF_ACC_PROVIDER_HOST=registry.opentofu.org make testacc
```

The `demo/` directory has a standalone configuration for trying each data
source.

## License

Copyright (C) 2025 John Long

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at
your option) any later version. See [LICENSE](LICENSE) for the full text.
