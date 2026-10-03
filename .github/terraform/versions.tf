terraform {
  # Import blocks with for_each require Terraform/OpenTofu 1.7+.
  required_version = ">= 1.7"

  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.13"
    }
  }
}

# Authenticates using the GITHUB_TOKEN environment variable, e.g.:
#   GITHUB_TOKEN=$(gh auth token) tofu plan
provider "github" {
  owner = local.owner
}

locals {
  owner      = "adduc"
  repository = "terraform-provider-docker"
}
