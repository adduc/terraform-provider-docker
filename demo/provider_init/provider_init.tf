terraform {
  required_providers {
    docker = {
      source = "adduc/docker"
    }
  }
}

provider "docker" {
}
