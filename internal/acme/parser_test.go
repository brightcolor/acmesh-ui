package acme

import (
	"encoding/base64"
	"testing"
)

func TestDecodeAcmeValue(t *testing.T) {
	plain := "systemctl reload nginx"
	wrapped := "__ACME_BASE64__START_" + base64.StdEncoding.EncodeToString([]byte(plain)) + "__ACME_BASE64__END_"
	if got := DecodeAcmeValue(wrapped); got != plain {
		t.Fatalf("decode wrapped: got %q want %q", got, plain)
	}
	// Plain values pass through unchanged.
	if got := DecodeAcmeValue(plain); got != plain {
		t.Fatalf("plain passthrough: got %q", got)
	}
	// Invalid base64 inside markers falls back to the raw value.
	bad := "__ACME_BASE64__START_!!!notb64!!!__ACME_BASE64__END_"
	if got := DecodeAcmeValue(bad); got != bad {
		t.Fatalf("bad base64 should pass through: got %q", got)
	}
}

func TestParseDomainConfDecodesReloadCmd(t *testing.T) {
	reload := "systemctl reload nginx && systemctl reload haproxy"
	wrapped := "__ACME_BASE64__START_" + base64.StdEncoding.EncodeToString([]byte(reload)) + "__ACME_BASE64__END_"
	conf := ParseDomainConf("Le_ReloadCmd='" + wrapped + "'\nLe_Webroot='dns_cf'\n")
	if conf["Le_ReloadCmd"] != reload {
		t.Fatalf("reload not decoded: got %q", conf["Le_ReloadCmd"])
	}
	if conf["Le_Webroot"] != "dns_cf" {
		t.Fatalf("plain value altered: got %q", conf["Le_Webroot"])
	}
}
