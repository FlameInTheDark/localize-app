//go:build windows

package runtime

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// configureHiddenChildProcess keeps Localize-owned console programs from
// flashing a terminal window while they run in the background.
func configureHiddenChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
