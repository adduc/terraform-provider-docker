package internal

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/moby/moby/client"
)

func TestAccServerVersionDataSource(t *testing.T) {
	testAccPreCheck(t)

	// Compare against what the daemon reports directly, so the test doesn't
	// depend on the Docker version installed on the machine running it.
	c, err := client.New(client.FromEnv)
	if err != nil {
		t.Fatalf("creating docker client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	want, err := c.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		t.Fatalf("reading server version: %v", err)
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("data.docker_server_version.test", "platform.name", want.Platform.Name),
		resource.TestCheckResourceAttr("data.docker_server_version.test", "components.%", strconv.Itoa(len(want.Components))),
	}
	for _, component := range want.Components {
		prefix := "components." + component.Name + "."
		checks = append(checks,
			resource.TestCheckResourceAttr("data.docker_server_version.test", prefix+"name", component.Name),
			resource.TestCheckResourceAttr("data.docker_server_version.test", prefix+"version", component.Version),
			resource.TestCheckResourceAttr("data.docker_server_version.test", prefix+"details.%", strconv.Itoa(len(component.Details))),
		)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "docker_server_version" "test" {}`,
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
		},
	})
}
