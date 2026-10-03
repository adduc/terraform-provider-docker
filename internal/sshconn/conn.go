package sshconn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// conn is a net.Conn backed by a command's stdin and stdout.
type conn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *tailBuffer

	closing  atomic.Bool
	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error
}

// newConn starts cmd and returns a connection to its stdio.
//
// The process is deliberately not tied to ctx: the HTTP client owns the
// connection's lifetime and may reuse it after the dial context is done.
func newConn(ctx context.Context, name string, args ...string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	setProcAttr(cmd)

	c := &conn{cmd: cmd, stderr: &tailBuffer{}, waitDone: make(chan struct{})}
	cmd.Stderr = c.stderr

	var err error
	if c.stdin, err = cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if c.stdout, err = cmd.StdoutPipe(); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}
	return c, nil
}

// wait reaps the process once and returns a channel closed when it exits.
// It must only be called after reads from stdout are finished, or while
// closing.
func (c *conn) wait() <-chan struct{} {
	c.waitOnce.Do(func() {
		go func() {
			c.waitErr = c.cmd.Wait()
			close(c.waitDone)
		}()
	})
	return c.waitDone
}

// processError turns a pipe error into a more useful one when the process
// has exited unsuccessfully, including what it wrote to stderr.
func (c *conn) processError(err error) error {
	if err == nil || c.closing.Load() {
		return err
	}

	select {
	case <-c.wait():
	case <-time.After(10 * time.Second):
		return fmt.Errorf("command %v did not exit after %v: stderr=%q", c.cmd.Args, err, c.stderr.String())
	}

	if c.waitErr == nil {
		return err
	}
	return fmt.Errorf(
		"command %v exited with %v; make sure the URL is valid and Docker 18.09 or later is installed on the remote host: stderr=%q",
		c.cmd.Args, c.waitErr, c.stderr.String(),
	)
}

func (c *conn) Read(p []byte) (int, error) {
	n, err := c.stdout.Read(p)
	return n, c.processError(err)
}

func (c *conn) Write(p []byte) (int, error) {
	n, err := c.stdin.Write(p)
	return n, c.processError(err)
}

// CloseWrite closes the process's stdin, signalling EOF to the remote end.
func (c *conn) CloseWrite() error {
	if err := c.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

// Close closes both pipes and terminates the process, waiting for it to exit.
func (c *conn) Close() error {
	if !c.closing.CompareAndSwap(false, true) {
		return nil
	}
	_ = c.stdin.Close()
	_ = c.stdout.Close()

	done := c.wait()
	select {
	case <-done:
		return nil
	default:
	}

	if runtime.GOOS == "windows" {
		_ = c.cmd.Process.Kill()
	} else {
		_ = c.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
			return nil
		case <-time.After(3 * time.Second):
			_ = c.cmd.Process.Kill()
		}
	}
	<-done
	return nil
}

func (c *conn) LocalAddr() net.Addr  { return dummyAddr("local") }
func (c *conn) RemoteAddr() net.Addr { return dummyAddr("remote") }

// Deadlines are not supported on pipes; the Docker client relies on its own
// timeouts instead.
func (c *conn) SetDeadline(time.Time) error      { return nil }
func (c *conn) SetReadDeadline(time.Time) error  { return nil }
func (c *conn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return "ssh" }
func (a dummyAddr) String() string  { return string(a) }

// tailBuffer keeps the most recent output written to it, for error messages.
type tailBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

const tailBufferMax = 4096

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if extra := b.buf.Len() - tailBufferMax; extra > 0 {
		b.buf.Next(extra)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
