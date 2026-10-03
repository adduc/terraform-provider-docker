package sshconn

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Run in a new session so ssh has no controlling terminal; this keeps
		// ProxyCommand and password prompts from hanging the provider.
		Setsid: true,
		// Don't leave ssh running if the provider process dies.
		Pdeathsig: syscall.SIGKILL,
	}
}
