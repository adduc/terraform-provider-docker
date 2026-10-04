package internal

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Docker's fixed-width RFC3339Nano timestamp, as written to logs.
var testAccLogTimestamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{9}Z$`)

func TestAccLogsDataSource(t *testing.T) {
	testAccPreCheck(t)

	// Lines chosen to break naive parsing of the multiplexed log stream:
	//   - "012345678\n" is a 10-byte frame, so its length header ends in
	//     0x0A (a newline) when timestamps are off.
	//   - 234 zeros + "\n" + a 31-byte timestamp prefix is a 266-byte
	//     (0x010A) frame when timestamps are on.
	//   - 100,000 x's is longer than bufio.Scanner's 64 KiB default and
	//     longer than Docker's 16 KiB partial-message limit.
	long := strings.Repeat("x", 100000)
	name := testAccRunContainer(t, false, "sh", "-c",
		`echo 012345678; printf '%0234d\n' 0; head -c 100000 /dev/zero | tr '\0' x; echo; echo to-stderr >&2`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "docker_logs" "test" {
					  container = %q
					}
				`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.#", "4"),
					resource.TestCheckTypeSetElemNestedAttrs("data.docker_logs.test", "logs.*", map[string]string{
						"message": "012345678",
						"stdout":  "true",
						"stderr":  "false",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.docker_logs.test", "logs.*", map[string]string{
						"message": strings.Repeat("0", 234),
						"stdout":  "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.docker_logs.test", "logs.*", map[string]string{
						"message": long,
						"stdout":  "true",
					}),
					resource.TestCheckTypeSetElemNestedAttrs("data.docker_logs.test", "logs.*", map[string]string{
						"message": "to-stderr",
						"stdout":  "false",
						"stderr":  "true",
					}),
					resource.TestMatchResourceAttr("data.docker_logs.test", "logs.0.timestamp", testAccLogTimestamp),
				),
			},
			{
				Config: fmt.Sprintf(`
					data "docker_logs" "test" {
					  container  = %q
					  timestamps = false
					}
				`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.#", "4"),
					resource.TestCheckTypeSetElemNestedAttrs("data.docker_logs.test", "logs.*", map[string]string{
						"message": "012345678",
						"stdout":  "true",
					}),
					resource.TestCheckNoResourceAttr("data.docker_logs.test", "logs.0.timestamp"),
				),
			},
		},
	})
}

func TestAccLogsDataSource_tty(t *testing.T) {
	testAccPreCheck(t)

	// A TTY container's log stream is raw output with no frame headers, and
	// lines end in \r\n.
	name := testAccRunContainer(t, true, "sh", "-c", "echo hello; echo world >&2")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "docker_logs" "test" {
					  container = %q
					}
				`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.#", "2"),
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.0.message", "hello"),
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.0.stdout", "true"),
					resource.TestCheckResourceAttr("data.docker_logs.test", "logs.1.message", "world"),
					resource.TestMatchResourceAttr("data.docker_logs.test", "logs.1.timestamp", testAccLogTimestamp),
				),
			},
		},
	})
}
