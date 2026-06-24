//go:build !linux

package updater

import "errors"

// Restart is not supported off Linux (production target). On other platforms
// restart the service manually after an update.
func Restart() error {
	return errors.New("automatic restart is only supported on Linux; restart the service manually")
}

// RestartSupported reports whether in-process restart is available.
func RestartSupported() bool { return false }
