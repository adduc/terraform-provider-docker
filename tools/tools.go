package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// Format Terraform code for use in documentation.
// If you do not have Terraform installed, you can remove the formatting command, but it is suggested
// to ensure the documentation is formatted properly.
//go:generate bash -c "terraform fmt -recursive ../examples/ || tofu fmt -recursive ../examples/"

// Generate documentation.
// Pin the Terraform version so every run generates docs with the same version.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name docker --tf-version 1.16.5
