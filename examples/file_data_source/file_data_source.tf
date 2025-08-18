terraform {
  required_providers {
    docker = {
      source = "adduc/docker"
    }
  }
}

provider "docker" {
  host = var.host
}

variable "container_name" {
  description = "The name of the container to retrieve logs for"
  type        = string
}

variable "path" {
  description = "The path to the file to retrieve from the container"
  type        = string
}

variable "host" {
  description = "The Docker host to connect to"
  type        = string
  default     = null
  nullable    = true
}

data "docker_file" "file" {
  container = var.container_name
  path      = var.path
}

output "file" {
  value     = data.docker_file.file
  sensitive = true
}
