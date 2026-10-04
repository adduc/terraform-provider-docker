terraform {
  required_providers {
    # Manages images, containers, networks, and volumes.
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 4.6"
    }

    # Reads files, logs, and server information. Both providers are named
    # "docker", so this one gets a different local name. The registry host
    # lets OpenTofu find it, since it isn't in the OpenTofu registry.
    adduc-docker = {
      source  = "registry.terraform.io/adduc/docker"
      version = "~> 0.0.6"
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
