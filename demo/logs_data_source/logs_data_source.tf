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

data "docker_logs" "logs" {
  container  = var.container_name
  timestamps = false
}

output "logs" {
  value = data.docker_logs.logs
}
