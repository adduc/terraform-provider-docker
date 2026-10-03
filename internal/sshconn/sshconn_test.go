package sshconn

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseURLAndArgs(t *testing.T) {
	base := []string{"-o", "ConnectTimeout=30", "-T"}
	tests := []struct {
		url  string
		args []string
	}{
		{
			url:  "ssh://example.com",
			args: append(base, "--", "example.com", "docker system dial-stdio"),
		},
		{
			url:  "ssh://me@example.com:2222",
			args: append(base, "-l", "me", "-p", "2222", "--", "example.com", "docker system dial-stdio"),
		},
		{
			url:  "ssh://example.com/",
			args: append(base, "--", "example.com", "docker system dial-stdio"),
		},
		{
			url:  "ssh://example.com/var/run/docker.sock",
			args: append(base, "--", "example.com", "docker --host=unix:///var/run/docker.sock system dial-stdio"),
		},
		{
			url:  "ssh://example.com/tmp/it's%20here;rm%20-rf",
			args: append(base, "--", "example.com", `docker '--host=unix:///tmp/it'\''s here;rm -rf' system dial-stdio`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			sp, err := ParseURL(tt.url)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			if got := sp.Args(); !reflect.DeepEqual(got, tt.args) {
				t.Errorf("Args()\n got: %q\nwant: %q", got, tt.args)
			}
		})
	}
}

func TestParseURLErrors(t *testing.T) {
	tests := map[string]string{
		"tcp://example.com":           "incorrect scheme",
		"ssh:example.com":             "expected ssh://",
		"ssh://":                      "hostname is empty",
		"ssh://me:secret@example.com": "password",
		"ssh://example.com?x=1":       "query parameters",
		"ssh://example.com#frag":      "fragments",
		"ssh://-oProxyCommand=x":      "host must not start with '-'",
		"ssh://-oProxyCommand=x@host": "user must not start with '-'",
	}

	for url, want := range tests {
		t.Run(url, func(t *testing.T) {
			_, err := ParseURL(url)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("ParseURL(%q) error = %v, want containing %q", url, err, want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"docker":               "docker",
		"--host=unix:///a/b.c": "--host=unix:///a/b.c",
		"":                     "''",
		"a b":                  "'a b'",
		"$(id)":                "'$(id)'",
		"it's":                 `'it'\''s'`,
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
