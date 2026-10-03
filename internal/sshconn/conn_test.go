//go:build unix

package sshconn

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestConnRoundTrip(t *testing.T) {
	c, err := newConn(context.Background(), "cat")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := c.(*conn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("read %q, want %q", got, "hello")
	}
}

func TestConnReportsStderrOnFailure(t *testing.T) {
	c, err := newConn(context.Background(), "sh", "-c", "echo boom >&2; exit 3")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = io.ReadAll(c)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"exit status 3", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestConnCloseTerminatesProcess(t *testing.T) {
	c, err := newConn(context.Background(), "sleep", "60")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- c.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
	if c.(*conn).cmd.ProcessState == nil {
		t.Fatal("process was not reaped")
	}
}

func TestNewConnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newConn(ctx, "cat"); err == nil {
		t.Fatal("expected error for canceled context")
	}
}
