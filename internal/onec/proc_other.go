//go:build !windows

package onec

import "os/exec"

func configureHiddenProcess(_ *exec.Cmd) {}
