//go:build !unix

package sshconn

import "os/exec"

func setProcAttr(*exec.Cmd) {}
