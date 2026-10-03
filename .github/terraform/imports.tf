# Adopt resources that existed before this configuration was introduced.
# Import blocks are idempotent, so these can stay in place after the first
# apply.

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
