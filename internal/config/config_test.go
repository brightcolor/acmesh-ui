package config

import "testing"

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
