package internal

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFileDataSources(t *testing.T) {
	testAccPreCheck(t)

	// bin.dat is not valid UTF-8, so it checks content_base64 is binary-safe.
	name := testAccRunContainer(t, false, "sh", "-c",
		`mkdir -p /tmp/t/dir /tmp/empty; printf hello > /tmp/t/text.txt; printf '\377\376\000\001' > /tmp/t/bin.dat; printf dots > /tmp/backup..old`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "docker_file" "text" {
					  container = %[1]q
					  path      = "/tmp/t/text.txt"
					}

					data "docker_file" "binary" {
					  container = %[1]q
					  path      = "/tmp/t/bin.dat"
					}

					data "docker_file" "dir" {
					  container = %[1]q
					  path      = "/tmp/empty"
					}

					# ".." inside a file name is not path traversal.
					data "docker_file" "dots" {
					  container = %[1]q
					  path      = "/tmp/backup..old"
					}

					# Relative paths resolve from the container's root.
					data "docker_file" "relative" {
					  container = %[1]q
					  path      = "tmp/t/text.txt"
					}

					data "docker_files" "all" {
					  container = %[1]q
					  path      = "/tmp/t"
					}
				`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.docker_file.text", "file.content_base64", "aGVsbG8="),
					resource.TestCheckResourceAttr("data.docker_file.text", "file.mode", "420"),
					resource.TestCheckResourceAttr("data.docker_file.text", "stat.mode", "420"),

					resource.TestCheckResourceAttr("data.docker_file.binary", "file.content_base64", "//4AAQ=="),
					resource.TestCheckResourceAttr("data.docker_file.binary", "file.size", "4"),

					// os.ModeDir (bit 31) | 0755: overflows an int32.
					resource.TestCheckResourceAttr("data.docker_file.dir", "stat.mode", "2147484141"),
					resource.TestCheckNoResourceAttr("data.docker_file.dir", "file.content_base64"),

					resource.TestCheckResourceAttr("data.docker_file.dots", "file.content_base64", "ZG90cw=="),
					resource.TestCheckResourceAttr("data.docker_file.relative", "file.content_base64", "aGVsbG8="),

					resource.TestCheckResourceAttr("data.docker_files.all", "files.%", "4"),
					resource.TestCheckResourceAttr("data.docker_files.all", "files.t/text.txt.content_base64", "aGVsbG8="),
					resource.TestCheckResourceAttr("data.docker_files.all", "files.t/bin.dat.content_base64", "//4AAQ=="),
					resource.TestCheckNoResourceAttr("data.docker_files.all", "files.t/dir/.content_base64"),
					resource.TestCheckResourceAttr("data.docker_files.all", "stat.mode", "2147484141"),
				),
			},
		},
	})
}
