locals {
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

resource "github_issue_label" "this" {
  for_each = local.labels

  repository  = github_repository.this.name
  name        = each.key
  color       = each.value.color
  description = each.value.description
}
