package internal

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// testAccImage is a small image with a POSIX shell, used to produce known
// container output.
const testAccImage = "busybox:latest"

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"docker": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck skips the test unless acceptance tests are enabled. It is
// called before any containers are created, so plain `go test` never needs a
// Docker daemon.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
}

// testAccRunContainer runs cmd in a new container, waits for it to exit, and
// returns the container's name. The container is removed when the test ends.
func testAccRunContainer(t *testing.T, tty bool, cmd ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := client.New(client.FromEnv)
	if err != nil {
		t.Fatalf("creating docker client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if _, err := c.ImageInspect(ctx, testAccImage); errdefs.IsNotFound(err) {
		pull, err := c.ImagePull(ctx, testAccImage, client.ImagePullOptions{})
		if err != nil {
			t.Fatalf("pulling %s: %v", testAccImage, err)
		}
		_, _ = io.Copy(io.Discard, pull)
		if err := pull.Wait(ctx); err != nil {
			t.Fatalf("pulling %s: %v", testAccImage, err)
		}
		_ = pull.Close()
	} else if err != nil {
		t.Fatalf("inspecting %s: %v", testAccImage, err)
	}

	created, err := c.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{Image: testAccImage, Cmd: cmd, Tty: tty},
	})
	if err != nil {
		t.Fatalf("creating container: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.ContainerRemove(context.Background(), created.ID, client.ContainerRemoveOptions{Force: true})
	})

	wait := c.ContainerWait(ctx, created.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNextExit})

	if _, err := c.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("starting container: %v", err)
	}

	select {
	case res := <-wait.Result:
		if res.StatusCode != 0 {
			t.Fatalf("container exited with status %d", res.StatusCode)
		}
	case err := <-wait.Error:
		t.Fatalf("waiting for container: %v", err)
	}

	inspect, err := c.ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspecting container: %v", err)
	}
	// Container names are reported with a leading slash.
	return inspect.Container.Name[1:]
}
