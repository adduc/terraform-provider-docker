package internal

import (
	"context"
	"os"
	"testing"

	"github.com/moby/moby/client"
)

// TestHostOptsSSHIntegration pings a real daemon over ssh. Set
// DOCKER_SSH_TEST_HOST (e.g. ssh://localhost) to a host reachable with
// passwordless ssh that has Docker installed.
func TestHostOptsSSHIntegration(t *testing.T) {
	host := os.Getenv("DOCKER_SSH_TEST_HOST")
	if host == "" {
		t.Skip("DOCKER_SSH_TEST_HOST not set")
	}

	opts, err := hostOpts(host)
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	v, err := c.ServerVersion(context.Background(), client.ServerVersionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("server version %s", v.Version)
}
