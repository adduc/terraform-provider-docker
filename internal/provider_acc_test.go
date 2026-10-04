package internal

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProviderTimeout(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					provider "docker" {
					  timeout = 0
					}

					data "docker_server_version" "test" {}
				`,
				ExpectError: regexp.MustCompile(`(?s)Attribute timeout value must be at least 1`),
			},
			{
				Config: `
					provider "docker" {
					  timeout = 5
					}

					data "docker_server_version" "test" {}
				`,
				Check: resource.TestCheckResourceAttrSet("data.docker_server_version.test", "platform.name"),
			},
		},
	})
}
