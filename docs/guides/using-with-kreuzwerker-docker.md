---
page_title: "Using with kreuzwerker/docker"
subcategory: ""
description: |-
  Read files and logs from containers managed by the kreuzwerker/docker provider.
---

# Using with kreuzwerker/docker

This provider only reads from Docker. To create the containers it reads from, use it alongside [kreuzwerker/docker](https://registry.terraform.io/providers/kreuzwerker/docker/latest), which manages images, containers, networks, and volumes.

## Naming the providers

Both providers are named `docker`, so one of them needs a different [local name](https://developer.hashicorp.com/terraform/language/providers/requirements#local-names). Resources and data sources use the provider whose local name matches the start of their type, so `docker_container` and `docker_file` both default to whichever provider is named `docker`. The examples below keep `docker` for kreuzwerker/docker and name this one `adduc-docker`:

```terraform
terraform {
  required_providers {
    docker = {
      source = "kreuzwerker/docker"
    }
    adduc-docker = {
      source = "adduc/docker"
    }
  }
}
```

Each data source from this provider then sets `provider = adduc-docker`. Without it, Terraform looks for the data source in kreuzwerker/docker, which has no `docker_file`, `docker_files`, or `docker_server_version`. It does have its own `docker_logs`, with different arguments and attributes, so errors about a `docker_logs` block usually mean the `provider` argument is missing.

## Reading from a container you create

Two things make sure the data sources read the container only after it is ready:

- **Pass the container's `id`, not its `name`.** The name is known while planning, so Terraform reads the data source during the plan, before the container exists. The ID is unknown until the container is created, which defers the read until after it.
- **Wait for the container to be ready.** A container that was just created may not have written its files yet. Give it a `healthcheck` and set `wait = true`, so kreuzwerker/docker doesn't finish creating it until it is healthy.

This example reads a token that a container generates on startup, along with its logs:

```terraform
terraform {
  required_providers {
    # Manages images, containers, networks, and volumes.
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 4.6"
    }

    # Reads files, logs, and server information. Both providers are named
    # "docker", so this one gets a different local name.
    adduc-docker = {
      source  = "adduc/docker"
      version = "~> 0.0.5"
    }
  }
}

provider "docker" {}

provider "adduc-docker" {}

resource "docker_image" "alpine" {
  name = "alpine:3.22"
}

resource "docker_container" "app" {
  name  = "app"
  image = docker_image.alpine.image_id

  # Generate a token on startup, as a service might generate credentials.
  command = [
    "sh", "-c",
    "head -c 32 /dev/urandom | base64 > /run/token && echo token generated && exec sleep infinity",
  ]

  # Don't consider the container created until the token exists, so the
  # data sources below never read it too early.
  wait = true
  healthcheck {
    test     = ["CMD", "test", "-s", "/run/token"]
    interval = "1s"
    retries  = 10
  }
}

data "docker_file" "token" {
  # Without this, Terraform uses the "docker" provider (kreuzwerker/docker).
  provider = adduc-docker

  # Use the container ID, not its name: the ID is unknown until the
  # container is created, which defers the read until after it exists.
  container = docker_container.app.id
  path      = "/run/token"
}

data "docker_logs" "app" {
  provider  = adduc-docker
  container = docker_container.app.id
}

output "token" {
  value     = trimspace(base64decode(data.docker_file.token.file.content_base64))
  sensitive = true
}

output "logs" {
  value = [for line in data.docker_logs.app.logs : line.message]
}
```

`file.content_base64` is sensitive, so outputs and values derived from it must be marked `sensitive` too.

## Configuring another provider from a container

A file read from a container can configure another provider in the same run. This example starts a [k3s](https://k3s.io/) cluster in a container, reads its kubeconfig, and uses that to deploy to the cluster with [alekc/kubectl](https://registry.terraform.io/providers/alekc/kubectl/latest):

```terraform
terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 4.6"
    }

    adduc-docker = {
      source  = "adduc/docker"
      version = "~> 0.0.5"
    }

    kubectl = {
      source  = "alekc/kubectl"
      version = "~> 2.4"
    }
  }
}

provider "docker" {}

provider "adduc-docker" {}

locals {
  kubeconfig = yamldecode(base64decode(data.docker_file.kubeconfig.file.content_base64))
}

provider "kubectl" {
  host                   = "https://127.0.0.1:${docker_container.k3s.ports[0].external}"
  cluster_ca_certificate = base64decode(local.kubeconfig.clusters[0].cluster["certificate-authority-data"])
  client_certificate     = base64decode(local.kubeconfig.users[0].user["client-certificate-data"])
  client_key             = base64decode(local.kubeconfig.users[0].user["client-key-data"])
  load_config_file       = false
}

resource "docker_image" "k3s" {
  name = "rancher/k3s:v1.32.1-k3s1"
}

resource "docker_container" "k3s" {
  name       = "k3s"
  image      = docker_image.k3s.image_id
  privileged = true

  entrypoint = ["sh", "-c"]
  command = [join(" && ", [
    # Avoids "path /var/lib/kubelet/pods is mounted on /var/lib/kubelet
    # but it is not a shared mount".
    # https://github.com/k3d-io/k3d/issues/1063#issuecomment-1153271637
    "mount --make-rshared /",
    "exec k3s server --disable=traefik,servicelb,metrics-server --node-name=k3s",
  ])]

  tmpfs = {
    "/run"     = ""
    "/var/run" = ""
  }

  ports {
    internal = 6443
    external = 6443
  }

  # Wait for the API server, so the kubeconfig exists before it is read.
  wait = true
  healthcheck {
    test     = ["CMD-SHELL", "kubectl get nodes"]
    interval = "2s"
    timeout  = "2s"
    retries  = 30
  }
}

data "docker_file" "kubeconfig" {
  provider  = adduc-docker
  container = docker_container.k3s.id
  path      = "/etc/rancher/k3s/k3s.yaml"
}

resource "kubectl_manifest" "nginx" {
  yaml_body = yamlencode({
    apiVersion = "apps/v1"
    kind       = "Deployment"
    metadata   = { name = "nginx", namespace = "default" }
    spec = {
      replicas = 1
      selector = { matchLabels = { app = "nginx" } }
      template = {
        metadata = { labels = { app = "nginx" } }
        spec = {
          containers = [{ name = "nginx", image = "nginx:1.29" }]
        }
      }
    }
  })
}
```

~> Configuring a provider from a resource created in the same run is fragile. If the container is replaced, or its kubeconfig can't be read, the kubectl provider is left unconfigured during the plan. For anything long-lived, create the cluster and deploy to it in separate configurations.
