//go:build !windows

package runtime

import "os/exec"

func configureHiddenChildProcess(_ *exec.Cmd) {}
