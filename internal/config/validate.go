package config

import (
	"fmt"
	"strings"
)

// Validate checks the configuration for internal consistency and enforces the
// security gate on open binding without authentication.
func Validate(c Config) error {
	switch c.Auth.Mode {
	case "none", "basic", "reverse_proxy":
	default:
		return fmt.Errorf("auth.mode %q is invalid (expected none|basic|reverse_proxy)", c.Auth.Mode)
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range", c.Server.Port)
	}

	// The core safety gate: binding to a non-loopback address without internal
	// auth requires an explicit opt-in.
	if c.IsOpenBind() && c.AuthDisabled() && !c.Security.AllowOpenWithoutAuth {
		return fmt.Errorf(
			"refusing to start: server.bind=%q is reachable from the network and auth.mode=none. "+
				"Set security.allow_open_without_auth=true to confirm, or bind to 127.0.0.1 and use a VPN/SSH tunnel/reverse proxy",
			c.Server.Bind)
	}

	if c.Acme.Binary == "" {
		return fmt.Errorf("acme.binary must be set")
	}
	if !strings.HasPrefix(c.Acme.Binary, "/") {
		return fmt.Errorf("acme.binary %q must be an absolute path", c.Acme.Binary)
	}

	// Reload templates must be argv form (never a shell string).
	for i, rc := range c.Reloads {
		if rc.Name == "" {
			return fmt.Errorf("reload_commands[%d]: name is required", i)
		}
		if len(rc.Command) == 0 {
			return fmt.Errorf("reload_commands[%d] %q: command must have at least one element", i, rc.Name)
		}
		for _, part := range rc.Command {
			if strings.ContainsAny(part, "\n\r\x00") {
				return fmt.Errorf("reload_commands[%d] %q: command part contains a control character", i, rc.Name)
			}
		}
	}
	return nil
}
