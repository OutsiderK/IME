//go:build windows

package rime

import (
	"os/exec"
	"syscall"
)

func configureLocalAICommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
