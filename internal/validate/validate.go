// Package validate contains strict input validation for everything that ends
// up in an acme.sh argument list: domains, wildcards, file paths, DNS provider
// codes and deploy hook names. The security model relies on these checks since
// commands are never built from shell strings.
package validate

import (
	"fmt"
	"strings"
)

// shellMeta lists characters that must never appear in user supplied values.
// Even though we never use a shell, rejecting them is defense in depth.
const shellMeta = "`$;&|<>(){}[]!\\\"'\n\r\t*?~"

// labelChars: a DNS label may contain a-z, 0-9 and hyphen.
func isLabelChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-'
}

// Domain validates a single FQDN. Wildcards are NOT allowed here; use Wildcard
// or DomainOrWildcard for those.
func Domain(d string) error {
	d = strings.TrimSpace(d)
	if d == "" {
		return fmt.Errorf("domain is empty")
	}
	if len(d) > 253 {
		return fmt.Errorf("domain %q is too long (max 253)", d)
	}
	if strings.ContainsAny(d, " ") {
		return fmt.Errorf("domain %q must not contain spaces", d)
	}
	if hasShellMeta(d) {
		return fmt.Errorf("domain %q contains forbidden characters", d)
	}
	if strings.Contains(d, "://") || strings.Contains(d, "/") {
		return fmt.Errorf("domain %q looks like a URL, expected a bare hostname", d)
	}
	if strings.HasPrefix(d, ".") || strings.HasSuffix(d, ".") {
		return fmt.Errorf("domain %q must not start or end with a dot", d)
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain %q must have at least two labels", d)
	}
	for _, l := range labels {
		if err := label(l); err != nil {
			return fmt.Errorf("domain %q: %w", d, err)
		}
	}
	return nil
}

func label(l string) error {
	if l == "" {
		return fmt.Errorf("empty label")
	}
	if len(l) > 63 {
		return fmt.Errorf("label %q too long (max 63)", l)
	}
	if strings.HasPrefix(l, "-") || strings.HasSuffix(l, "-") {
		return fmt.Errorf("label %q must not start or end with a hyphen", l)
	}
	for _, r := range l {
		if !isLabelChar(r) {
			return fmt.Errorf("label %q contains invalid character %q", l, r)
		}
	}
	return nil
}

// Wildcard validates a wildcard domain of the exact form *.example.com.
func Wildcard(d string) error {
	d = strings.TrimSpace(d)
	if !strings.HasPrefix(d, "*.") {
		return fmt.Errorf("wildcard %q must start with '*.'", d)
	}
	rest := d[2:]
	if strings.Contains(rest, "*") {
		return fmt.Errorf("wildcard %q may only contain a single leading '*'", d)
	}
	return Domain(rest)
}

// DomainOrWildcard accepts either a plain FQDN or a *.example.com wildcard.
func DomainOrWildcard(d string) error {
	if strings.HasPrefix(strings.TrimSpace(d), "*.") {
		return Wildcard(d)
	}
	return Domain(d)
}

// Domains validates a list and rejects empties and duplicates. At least one
// domain is required.
func Domains(domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}
	seen := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if err := DomainOrWildcard(d); err != nil {
			return err
		}
		key := strings.ToLower(d)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate domain %q", d)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// AbsolutePath validates a filesystem path used as a webroot, cert file etc.
// It requires an absolute Unix path and rejects shell constructs and null
// bytes. If allowedBases is non-empty, the path must live under one of them.
func AbsolutePath(p string, allowedBases []string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.ContainsRune(p, '\x00') {
		return fmt.Errorf("path contains a null byte")
	}
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q must be absolute", p)
	}
	for _, bad := range []string{"$(", "${", "`", "&&", "||", ";", "|", "\n", "\r"} {
		if strings.Contains(p, bad) {
			return fmt.Errorf("path %q contains a forbidden construct %q", p, bad)
		}
	}
	if strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") {
		return fmt.Errorf("path %q must not contain '..' traversal", p)
	}
	if len(allowedBases) > 0 {
		ok := false
		for _, base := range allowedBases {
			base = strings.TrimRight(strings.TrimSpace(base), "/")
			if base == "" {
				continue
			}
			if p == base || strings.HasPrefix(p, base+"/") {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("path %q is not under an allowed base directory", p)
		}
	}
	return nil
}

// ProviderCode validates an acme.sh DNS provider code such as dns_cf.
func ProviderCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("provider code is empty")
	}
	if !strings.HasPrefix(code, "dns_") {
		return fmt.Errorf("provider code %q must start with 'dns_'", code)
	}
	if len(code) > 64 {
		return fmt.Errorf("provider code %q too long", code)
	}
	for _, r := range code {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_'
		if !ok {
			return fmt.Errorf("provider code %q contains invalid character %q", code, r)
		}
	}
	return nil
}

// HookName validates a deploy hook name (acme.sh --deploy-hook value).
func HookName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("deploy hook name is empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("deploy hook %q too long", name)
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-'
		if !ok {
			return fmt.Errorf("deploy hook %q contains invalid character %q", name, r)
		}
	}
	return nil
}

// EnvName validates an environment variable name for DNS providers.
func EnvName(name string) error {
	if name == "" {
		return fmt.Errorf("env name is empty")
	}
	for i, r := range name {
		first := i == 0
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_'
		if !first {
			ok = ok || (r >= '0' && r <= '9')
		}
		if !ok {
			return fmt.Errorf("env name %q contains invalid character %q", name, r)
		}
	}
	return nil
}

// EnvValue rejects values that would break out of a single env assignment.
// Newlines and null bytes are never acceptable inside an env value.
func EnvValue(v string) error {
	if strings.ContainsAny(v, "\x00\n\r") {
		return fmt.Errorf("env value contains a newline or null byte")
	}
	return nil
}

// KeyType validates an acme.sh key type.
func KeyType(kt string) error {
	switch kt {
	case "", "ec-256", "ec-384", "ec-521", "2048", "3072", "4096":
		return nil
	default:
		return fmt.Errorf("unsupported key type %q", kt)
	}
}

func hasShellMeta(s string) bool {
	return strings.ContainsAny(s, shellMeta)
}
