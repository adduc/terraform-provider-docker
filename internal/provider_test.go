package internal

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adduc/terraform-provider-docker/internal/sshconn"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
			defer func() { _ = c.Close() }()
		})
	}
}

func TestHostOptsInvalidSSH(t *testing.T) {
	if _, err := hostOpts("ssh://-oProxyCommand=x"); err == nil {
		t.Fatal("expected error for invalid ssh host")
	}
}

// clearDockerEnv unsets the environment variables clientOpts reads, so tests
// don't depend on the environment they run in.
func clearDockerEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		client.EnvOverrideHost,
		client.EnvOverrideCertPath,
		client.EnvTLSVerify,
		client.EnvOverrideAPIVersion,
	} {
		t.Setenv(name, "")
	}
}

func newTestClient(t *testing.T, data ProviderModel) *client.Client {
	t.Helper()
	opts, err := clientOpts(data)
	if err != nil {
		t.Fatalf("clientOpts: %v", err)
	}
	c, err := client.New(opts...)
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClientOptsHost(t *testing.T) {
	tests := []struct {
		name string
		env  string
		attr string
		want string
	}{
		{name: "default", want: client.DefaultDockerHost},
		{name: "environment", env: "tcp://env.example.com:2375", want: "tcp://env.example.com:2375"},
		{name: "attribute", attr: "tcp://attr.example.com:2375", want: "tcp://attr.example.com:2375"},
		{name: "attribute overrides environment", env: "tcp://env.example.com:2375", attr: "tcp://attr.example.com:2375", want: "tcp://attr.example.com:2375"},
		// ssh:// hosts from the environment must use the ssh dialer, which
		// replaces the client's host with a placeholder.
		{name: "ssh from environment", env: "ssh://me@example.com", want: sshconn.Host},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearDockerEnv(t)
			t.Setenv(client.EnvOverrideHost, tt.env)

			c := newTestClient(t, ProviderModel{Host: types.StringValue(tt.attr)})

			if got := c.DaemonHost(); got != tt.want {
				t.Errorf("DaemonHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientOptsTLS(t *testing.T) {
	// A stand-in daemon that only accepts TLS connections with a client
	// certificate.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Api-Version", "1.50")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"99.0.0","ApiVersion":"1.50"}`))
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	certPath := t.TempDir()
	writePEM(t, filepath.Join(certPath, "ca.pem"), "CERTIFICATE", srv.Certificate().Raw)
	writeClientCert(t, certPath)

	host := "tcp://" + srv.Listener.Addr().String()

	tests := []struct {
		name    string
		env     map[string]string
		data    ProviderModel
		wantErr bool
	}{
		{
			name: "attribute",
			data: ProviderModel{Host: types.StringValue(host), CertPath: types.StringValue(certPath)},
		},
		{
			name: "environment",
			env: map[string]string{
				client.EnvOverrideHost:     host,
				client.EnvOverrideCertPath: certPath,
				client.EnvTLSVerify:        "1",
			},
		},
		{
			name:    "no certificates",
			data:    ProviderModel{Host: types.StringValue(host)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearDockerEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			c := newTestClient(t, tt.data)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			v, err := c.ServerVersion(ctx, client.ServerVersionOptions{})
			if tt.wantErr {
				if err == nil {
					t.Fatal("ServerVersion succeeded without TLS certificates")
				}
				return
			}
			if err != nil {
				t.Fatalf("ServerVersion: %v", err)
			}
			if v.Version != "99.0.0" {
				t.Errorf("Version = %q, want 99.0.0", v.Version)
			}
		})
	}
}

func TestClientOptsMissingCerts(t *testing.T) {
	clearDockerEnv(t)

	opts, err := clientOpts(ProviderModel{
		Host:     types.StringValue("tcp://127.0.0.1:2376"),
		CertPath: types.StringValue(filepath.Join(t.TempDir(), "missing")),
	})
	if err != nil {
		t.Fatalf("clientOpts: %v", err)
	}
	if _, err := client.New(opts...); err == nil {
		t.Fatal("client.New succeeded with a missing cert_path")
	}
}

// writeClientCert writes a self-signed client certificate and key to dir as
// cert.pem and key.pem.
func writeClientCert(t *testing.T, dir string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	writePEM(t, filepath.Join(dir, "cert.pem"), "CERTIFICATE", der)
	writePEM(t, filepath.Join(dir, "key.pem"), "PRIVATE KEY", keyDER)
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
