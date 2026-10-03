package internal

import (
	"testing"

	"github.com/moby/moby/client"
)

func TestHostOpts(t *testing.T) {
	hosts := []string{
		"tcp://127.0.0.1:2375",
		"unix:///var/run/docker.sock",
		"ssh://me@example.com:2222",
	}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			opts, err := hostOpts(host)
			if err != nil {
				t.Fatalf("hostOpts: %v", err)
			}
			c, err := client.New(opts...)
			if err != nil {
				t.Fatalf("client.New: %v", err)
			}
			defer c.Close()
		})
	}
}

func TestHostOptsInvalidSSH(t *testing.T) {
	if _, err := hostOpts("ssh://-oProxyCommand=x"); err == nil {
		t.Fatal("expected error for invalid ssh host")
	}
}
