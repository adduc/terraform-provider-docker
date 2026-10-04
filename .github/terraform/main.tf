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

  labels = {
    # GitHub's default labels.
    "bug"              = { color = "d73a4a", description = "Something isn't working" }
    "documentation"    = { color = "0075ca", description = "Improvements or additions to documentation" }
    "duplicate"        = { color = "cfd3d7", description = "This issue or pull request already exists" }
    "enhancement"      = { color = "a2eeef", description = "New feature or request" }
    "good first issue" = { color = "7057ff", description = "Good for newcomers" }
    "help wanted"      = { color = "008672", description = "Extra attention is needed" }
    "invalid"          = { color = "e4e669", description = "This doesn't seem right" }
    "question"         = { color = "d876e3", description = "Further information is requested" }
    "wontfix"          = { color = "ffffff", description = "This will not be worked on" }

    # Used to categorize release notes (see .github/release.yml).
    "breaking-change"    = { color = "b60205", description = "Introduces a backwards-incompatible change" }
    "dependencies"       = { color = "0366d6", description = "Updates a dependency" }
    "ignore-for-release" = { color = "ededed", description = "Excluded from release notes" }
  }
}

################################################################################
# Repository
################################################################################

resource "github_repository" "this" {
  name       = local.repository
  visibility = "public"

  has_issues      = true
  has_projects    = true
  has_wiki        = true
  has_discussions = false

  # Only allow squash merges.
  allow_merge_commit     = false
  allow_squash_merge     = true
  allow_rebase_merge     = false
  allow_auto_merge       = false
  allow_update_branch    = false
  delete_branch_on_merge = false

  merge_commit_title          = "MERGE_MESSAGE"
  merge_commit_message        = "PR_TITLE"
  squash_merge_commit_title   = "COMMIT_OR_PR_TITLE"
  squash_merge_commit_message = "COMMIT_MESSAGES"

  security_and_analysis {
    secret_scanning {
      status = "enabled"
    }
    secret_scanning_push_protection {
      status = "enabled"
    }
  }

  # Archive rather than delete the repository if it is ever removed from
  # this configuration.
  archive_on_destroy = true

  lifecycle {
    prevent_destroy = true
  }
}

resource "github_branch_default" "this" {
  repository = github_repository.this.name
  branch     = "main"
}

################################################################################
# Labels
################################################################################

resource "github_issue_label" "this" {
  for_each = local.labels

  repository  = github_repository.this.name
  name        = each.key
  color       = each.value.color
  description = each.value.description
}

################################################################################
# Imports
#
# Adopt resources that existed before this configuration was introduced.
# Import blocks are idempotent, so these can stay in place after the first
# apply.
################################################################################

import {
  to = github_repository.this
  id = local.repository
}

import {
  to = github_branch_default.this
  id = local.repository
}

import {
  for_each = toset([
    "bug",
    "documentation",
    "duplicate",
    "enhancement",
    "good first issue",
    "help wanted",
    "invalid",
    "question",
    "wontfix",
  ])

  to = github_issue_label.this[each.key]
  id = "${local.repository}:${each.key}"
}
