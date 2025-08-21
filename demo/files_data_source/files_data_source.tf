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
  description = "The filepath to retrieve from the container"
  type        = string
}

data "docker_files" "files" {
  container = var.container_name
  path      = var.path
}

output "files" {
  value     = data.docker_files.files
  sensitive = true
}
