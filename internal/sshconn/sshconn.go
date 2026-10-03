// Package sshconn connects to a remote Docker daemon over ssh:// URLs by
// running `docker system dial-stdio` on the remote host through the local
// ssh binary, mirroring the behavior of the Docker CLI's connection helper.
package sshconn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Host is a placeholder used as the Docker client's host when dialing over
// ssh. The dialer ignores the address, so it only needs to form valid URLs.
const Host = "http://docker.example.com"

// DialFunc matches the signature expected by client.WithDialContext.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Spec holds the parts of an ssh:// URL used to build the ssh command.
type Spec struct {
	User string
	Host string
	Port string
	Path string
}

// ParseURL validates an ssh:// URL and returns its Spec.
func ParseURL(daemonURL string) (*Spec, error) {
	u, err := url.Parse(daemonURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH URL: %w", err)
	}
	if u.Scheme != "ssh" {
		return nil, fmt.Errorf("invalid SSH URL: incorrect scheme %q", u.Scheme)
	}
	if u.Opaque != "" {
		return nil, errors.New("invalid SSH URL: expected ssh://[user@]host[:port][/path]")
	}

	var sp Spec
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			return nil, errors.New("invalid SSH URL: plain-text password is not supported")
		}
		sp.User = u.User.Username()
	}
	sp.Host = u.Hostname()
	sp.Port = u.Port()
	sp.Path = u.Path

	switch {
	case sp.Host == "":
		return nil, errors.New("invalid SSH URL: hostname is empty")
	case u.RawQuery != "":
		return nil, fmt.Errorf("invalid SSH URL: query parameters are not allowed: %q", u.RawQuery)
	case u.Fragment != "":
		return nil, fmt.Errorf("invalid SSH URL: fragments are not allowed: %q", u.Fragment)
	}

	// Values from the URL are passed to ssh as arguments, so make sure none of
	// them can be mistaken for an option.
	for name, v := range map[string]string{"user": sp.User, "host": sp.Host, "port": sp.Port} {
		if strings.HasPrefix(v, "-") {
			return nil, fmt.Errorf("invalid SSH URL: %s must not start with '-': %q", name, v)
		}
	}

	return &sp, nil
}

// Args returns the arguments (excluding "ssh" itself) that run
// `docker system dial-stdio` on the remote host.
func (sp *Spec) Args() []string {
	args := []string{"-o", "ConnectTimeout=30", "-T"}
	if sp.User != "" {
		args = append(args, "-l", sp.User)
	}
	if sp.Port != "" {
		args = append(args, "-p", sp.Port)
	}

	remote := []string{"docker", "system", "dial-stdio"}
	if strings.Trim(sp.Path, "/") != "" {
		remote = []string{"docker", "--host=unix://" + sp.Path, "system", "dial-stdio"}
	}

	// ssh joins the remote command into a single string that the remote
	// user's shell parses, so each word must be quoted.
	quoted := make([]string, len(remote))
	for i, w := range remote {
		quoted[i] = shellQuote(w)
	}

	return append(args, "--", sp.Host, strings.Join(quoted, " "))
}

// NewDialer returns a DialFunc that connects to the Docker daemon described
// by an ssh:// URL.
func NewDialer(daemonURL string) (DialFunc, error) {
	sp, err := ParseURL(daemonURL)
	if err != nil {
		return nil, err
	}
	args := sp.Args()
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return newConn(ctx, "ssh", args...)
	}, nil
}

// shellQuote quotes s for a POSIX shell, leaving it bare when it only
// contains characters that are never special.
func shellQuote(s string) string {
	safe := s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("-_./:=@%+,", r))
	}) < 0
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
