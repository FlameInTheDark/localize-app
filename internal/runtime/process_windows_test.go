//go:build windows

package runtime

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureHiddenChildProcess(t *testing.T) {
	command := exec.Command("llama-server.exe")
	configureHiddenChildProcess(command)

	if command.SysProcAttr == nil {
		t.Fatal("expected Windows process attributes")
	}
	if !command.SysProcAttr.HideWindow {
		t.Error("expected the child window to be hidden")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Error("expected CREATE_NO_WINDOW to be set")
	}
}
