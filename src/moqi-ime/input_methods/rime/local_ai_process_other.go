//go:build !windows

package rime

import "os/exec"

func configureLocalAICommand(_ *exec.Cmd) {}
