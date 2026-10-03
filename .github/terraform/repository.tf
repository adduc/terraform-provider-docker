resource "github_repository" "this" {
  name       = local.repository
  visibility = "public"

  has_issues      = true
  has_projects    = true
  has_wiki        = true
  has_discussions = false

  allow_merge_commit     = true
  allow_squash_merge     = true
  allow_rebase_merge     = true
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
