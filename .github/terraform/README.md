# Repository configuration

This directory manages the GitHub repository's settings and labels using the
[`integrations/github`](https://search.opentofu.org/provider/integrations/github/latest)
provider.

## Usage

```sh
cd .github/terraform
tofu init
GITHUB_TOKEN=$(gh auth token) tofu plan
GITHUB_TOKEN=$(gh auth token) tofu apply
```

`terraform` can be used in place of `tofu`.

State is stored locally and is not committed (see the repository's
`.gitignore`).

## Adopting existing resources

Resources that already exist on GitHub are adopted via `import` blocks at the
end of `main.tf`. When adding a resource that already exists (e.g. a label
created through the web UI), add a matching import block so it is adopted
rather than recreated.
