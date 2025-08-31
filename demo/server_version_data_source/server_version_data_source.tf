terraform {
  required_providers {
    docker = {
      source = "adduc/docker"
    }
  }
}

provider "docker" {
}

data "docker_server_version" "server_version" {
}

output "server_version" {
  value = data.docker_server_version.server_version
}
