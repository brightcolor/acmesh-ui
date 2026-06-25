package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestJSONKeysSnakeCase guards against the settings page showing "undefined":
// the config structs must serialize with snake_case JSON keys (not Go field
// names), because the frontend reads e.g. server.bind / jobs.max_parallel.
func TestJSONKeysSnakeCase(t *testing.T) {
	data, err := json.Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, key := range []string{
		`"bind"`, `"port"`, `"mode"`, `"binary"`, `"home"`,
		`"default_ca"`, `"default_key_type"`, `"max_parallel"`,
		`"timeout_seconds"`, `"log_retention_days"`, `"expiring_soon_days"`,
		`"name"`, `"command"`,
	} {
		if !strings.Contains(s, key) {
			t.Errorf("expected config JSON to contain key %s; got: %s", key, s)
		}
	}
	// Must NOT leak Go field names.
	for _, bad := range []string{`"Bind"`, `"MaxParallel"`, `"DefaultKeyType"`} {
		if strings.Contains(s, bad) {
			t.Errorf("config JSON leaked Go field name %s", bad)
		}
	}
}

func TestValidateOpenBindGate(t *testing.T) {
	c := Default()
	c.Server.Bind = "0.0.0.0"
	c.Auth.Mode = "none"
	c.Security.AllowOpenWithoutAuth = false
	if err := Validate(c); err == nil {
		t.Fatalf("expected open bind without auth to be rejected")
	}

	c.Security.AllowOpenWithoutAuth = true
	if err := Validate(c); err != nil {
		t.Fatalf("open bind with explicit opt-in should pass: %v", err)
	}
}

func TestValidateLoopbackAlwaysOK(t *testing.T) {
	c := Default()
	c.Server.Bind = "127.0.0.1"
	c.Auth.Mode = "none"
	if err := Validate(c); err != nil {
		t.Fatalf("loopback + none should pass: %v", err)
	}
}

func TestValidateAuthModeWithOpenBind(t *testing.T) {
	c := Default()
	c.Server.Bind = "10.0.0.5"
	c.Auth.Mode = "basic" // auth enabled => allowed even on open bind
	if err := Validate(c); err != nil {
		t.Fatalf("open bind with auth should pass: %v", err)
	}
}

func TestIsOpenBind(t *testing.T) {
	for _, b := range []string{"127.0.0.1", "::1", "localhost", ""} {
		c := Default()
		c.Server.Bind = b
		if c.IsOpenBind() {
			t.Errorf("%q should be loopback", b)
		}
	}
	for _, b := range []string{"0.0.0.0", "10.0.0.1", "192.168.1.5"} {
		c := Default()
		c.Server.Bind = b
		if !c.IsOpenBind() {
			t.Errorf("%q should be open", b)
		}
	}
}

func TestValidateReloadCommands(t *testing.T) {
	c := Default()
	c.Reloads = []ReloadCommand{{Name: "bad", Command: nil}}
	if err := Validate(c); err == nil {
		t.Fatalf("empty reload command should fail")
	}
}

func TestValidateInvalidAuthMode(t *testing.T) {
	c := Default()
	c.Auth.Mode = "ldap"
	if err := Validate(c); err == nil {
		t.Fatalf("invalid auth mode should fail")
	}
}
