//go:build unix && !linux

package sshconn

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	// Run in a new session so ssh has no controlling terminal; this keeps
	// ProxyCommand and password prompts from hanging the provider.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
