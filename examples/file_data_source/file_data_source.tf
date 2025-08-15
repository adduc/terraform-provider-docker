terraform {
  required_providers {
    docker = {
      source = "adduc/docker"
    }
  }
}

provider "docker" {
}

variable "container_name" {
  description = "The name of the container to retrieve logs for"
  type        = string
}

variable "path" {
  description = "The path to the file to retrieve from the container"
  type        = string
}

data "docker_file" "file" {
  container = var.container_name
  path      = var.path
}

output "content" {
  value = nonsensitive(data.docker_file.file.content)
}
