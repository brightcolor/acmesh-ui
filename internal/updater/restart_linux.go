//go:build linux

package updater

import (
	"os"
	"syscall"
)

// Restart replaces the current process image with the (freshly updated) binary
// via execve, keeping the same PID. Listener sockets are close-on-exec, so the
// port is released and re-bound by the new image during its normal startup.
func Restart() error {
	exe, err := ExecutablePath()
	if err != nil {
		return err
	}
	return syscall.Exec(exe, os.Args, os.Environ())
}

// RestartSupported reports whether in-process restart is available.
func RestartSupported() bool { return true }
