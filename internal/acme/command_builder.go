// Package acme orchestrates the external acme.sh CLI. Every invocation is built
// here from a fixed set of allowed actions and strictly validated arguments.
// Commands are ALWAYS argv slices - never shell strings - and the only
// executable ever run is the configured acme.sh binary.
package acme

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bright-color/acmesh-ui/internal/validate"
)

// Challenge enumerates the supported validation methods.
type Challenge string

const (
	ChallengeWebroot    Challenge = "webroot"
	ChallengeStandalone Challenge = "standalone"
	ChallengeDNS        Challenge = "dns"        // dns-01 via acme.sh DNS API
	ChallengeDNSManual  Challenge = "dns-manual" // not suitable for auto-renew
)

// Command is a fully built, validated acme.sh invocation. Args never includes
// the binary path itself. Env carries DNS provider variables (may contain
// secrets) and is applied to the process environment, not the argv.
type Command struct {
	Action string   // logical action name, e.g. "issue"
	Args   []string // argv passed to acme.sh (excluding the binary)
	Env    []string // KEY=VALUE pairs for the process environment
}

// PreviewArgs returns the binary + args as a copy-pastable command line. It is
// the caller's responsibility to run it through a secret Masker before display
// (Env values are never included here, but env var names are appended as a hint).
func (c Command) PreviewArgs(binary string) string {
	parts := append([]string{binary}, c.Args...)
	return strings.Join(quoteAll(parts), " ")
}

func quoteAll(parts []string) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		if strings.ContainsAny(p, " \t\"'") {
			out[i] = `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
		} else {
			out[i] = p
		}
	}
	return out
}

// Builder produces allowed acme.sh commands. It captures defaults from config.
type Builder struct {
	DefaultCA      string
	DefaultKeyType string
}

// List builds `acme.sh --list`.
func (b Builder) List() Command {
	return Command{Action: "list", Args: []string{"--list"}}
}

// Info builds `acme.sh --info -d <domain>`.
func (b Builder) Info(domain string) (Command, error) {
	if err := validate.DomainOrWildcard(domain); err != nil {
		return Command{}, err
	}
	return Command{Action: "info", Args: []string{"--info", "-d", domain}}, nil
}

// Version builds `acme.sh --version`.
func (b Builder) Version() Command {
	return Command{Action: "version", Args: []string{"--version"}}
}

// IssueSpec describes a certificate issuance request.
type IssueSpec struct {
	Domains   []string // first entry is the main domain
	Challenge Challenge
	Webroot   string            // for ChallengeWebroot
	DNSCode   string            // for ChallengeDNS, e.g. dns_cf
	DNSEnv    map[string]string // env vars for the DNS provider (secrets allowed)
	KeyType   string
	CA        string
	Staging   bool
	Force     bool
}

// Issue builds an `acme.sh --issue ...` command for the given spec.
func (b Builder) Issue(s IssueSpec) (Command, error) {
	if err := validate.Domains(s.Domains); err != nil {
		return Command{}, err
	}
	kt := s.KeyType
	if kt == "" {
		kt = b.DefaultKeyType
	}
	if err := validate.KeyType(kt); err != nil {
		return Command{}, err
	}

	args := []string{"--issue"}
	for _, d := range s.Domains {
		args = append(args, "-d", strings.TrimSpace(d))
	}

	switch s.Challenge {
	case ChallengeWebroot:
		if err := validate.AbsolutePath(s.Webroot, nil); err != nil {
			return Command{}, fmt.Errorf("webroot: %w", err)
		}
		args = append(args, "-w", s.Webroot)
	case ChallengeStandalone:
		args = append(args, "--standalone")
	case ChallengeDNS:
		if err := validate.ProviderCode(s.DNSCode); err != nil {
			return Command{}, err
		}
		args = append(args, "--dns", s.DNSCode)
	case ChallengeDNSManual:
		args = append(args, "--dns", "--yes-I-know-dns-manual-mode-enough-go-ahead-please")
	default:
		return Command{}, fmt.Errorf("unsupported challenge method %q", s.Challenge)
	}

	if kt != "" {
		args = append(args, "--keylength", kt)
	}
	if s.CA != "" {
		if err := validateCAName(s.CA); err != nil {
			return Command{}, err
		}
		args = append(args, "--server", s.CA)
	}
	if s.Staging {
		args = append(args, "--staging")
	}
	if s.Force {
		args = append(args, "--force")
	}

	cmd := Command{Action: "issue", Args: args}
	env, err := buildEnv(s.Challenge, s.DNSEnv)
	if err != nil {
		return Command{}, err
	}
	cmd.Env = env
	return cmd, nil
}

// Renew builds `acme.sh --renew -d <domain> [--force]`.
func (b Builder) Renew(domain string, force bool) (Command, error) {
	if err := validate.DomainOrWildcard(domain); err != nil {
		return Command{}, err
	}
	args := []string{"--renew", "-d", domain}
	if force {
		args = append(args, "--force")
	}
	action := "renew"
	if force {
		action = "force-renew"
	}
	return Command{Action: action, Args: args}, nil
}

// RenewAll builds `acme.sh --renew-all`.
func (b Builder) RenewAll() Command {
	return Command{Action: "renew-all", Args: []string{"--renew-all"}}
}

// Remove builds `acme.sh --remove -d <domain> [--ecc]`. This stops acme.sh from
// managing/renewing the certificate; the on-disk files are left untouched (the
// caller decides whether to purge the domain directory separately).
func (b Builder) Remove(domain string, ecc bool) (Command, error) {
	if err := validate.DomainOrWildcard(domain); err != nil {
		return Command{}, err
	}
	args := []string{"--remove", "-d", domain}
	if ecc {
		args = append(args, "--ecc")
	}
	return Command{Action: "remove", Args: args}, nil
}

// InstallSpec describes an --install-cert request.
type InstallSpec struct {
	Domain        string
	KeyFile       string
	FullchainFile string
	CertFile      string
	CAFile        string
	ReloadCmd     []string // allow-listed argv reload template (may be empty)
}

// InstallCert builds `acme.sh --install-cert ...`.
//
// allowedBases optionally restricts the install target paths. reloadAllowed
// reports whether the supplied reload template was matched against the
// configured allow-list; an unverified reload command is rejected.
func (b Builder) InstallCert(s InstallSpec, allowedBases []string, reloadAllowed bool) (Command, error) {
	if err := validate.DomainOrWildcard(s.Domain); err != nil {
		return Command{}, err
	}
	args := []string{"--install-cert", "-d", s.Domain}

	add := func(flag, path string) error {
		if path == "" {
			return nil
		}
		if err := validate.AbsolutePath(path, allowedBases); err != nil {
			return fmt.Errorf("%s: %w", flag, err)
		}
		args = append(args, flag, path)
		return nil
	}
	if err := add("--key-file", s.KeyFile); err != nil {
		return Command{}, err
	}
	if err := add("--fullchain-file", s.FullchainFile); err != nil {
		return Command{}, err
	}
	if err := add("--cert-file", s.CertFile); err != nil {
		return Command{}, err
	}
	if err := add("--ca-file", s.CAFile); err != nil {
		return Command{}, err
	}

	if len(s.ReloadCmd) > 0 {
		if !reloadAllowed {
			return Command{}, fmt.Errorf("reload command is not in the allowed template list")
		}
		// acme.sh expects a single string for --reloadcmd. We join the
		// allow-listed argv with spaces; because every element came from a
		// trusted template (validated by the caller) this cannot inject a
		// new command.
		for _, part := range s.ReloadCmd {
			if strings.ContainsAny(part, "\n\r\x00") {
				return Command{}, fmt.Errorf("reload command contains a control character")
			}
		}
		args = append(args, "--reloadcmd", strings.Join(s.ReloadCmd, " "))
	}
	return Command{Action: "install-cert", Args: args}, nil
}

// Deploy builds `acme.sh --deploy -d <domain> --deploy-hook <hook>`.
func (b Builder) Deploy(domain, hook string, env map[string]string) (Command, error) {
	if err := validate.DomainOrWildcard(domain); err != nil {
		return Command{}, err
	}
	if err := validate.HookName(hook); err != nil {
		return Command{}, err
	}
	args := []string{"--deploy", "-d", domain, "--deploy-hook", hook}
	cmd := Command{Action: "deploy", Args: args}
	envList, err := envPairs(env)
	if err != nil {
		return Command{}, err
	}
	cmd.Env = envList
	return cmd, nil
}

// SetDefaultCA builds `acme.sh --set-default-ca --server <ca>`.
func (b Builder) SetDefaultCA(ca string) (Command, error) {
	if err := validateCAName(ca); err != nil {
		return Command{}, err
	}
	return Command{Action: "set-default-ca", Args: []string{"--set-default-ca", "--server", ca}}, nil
}

// buildEnv produces the env list for DNS-based challenges.
func buildEnv(ch Challenge, env map[string]string) ([]string, error) {
	if ch != ChallengeDNS {
		return nil, nil
	}
	return envPairs(env)
}

func envPairs(env map[string]string) ([]string, error) {
	if len(env) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic for previews/tests
	out := make([]string, 0, len(env))
	for _, k := range keys {
		if err := validate.EnvName(k); err != nil {
			return nil, err
		}
		if err := validate.EnvValue(env[k]); err != nil {
			return nil, err
		}
		out = append(out, k+"="+env[k])
	}
	return out, nil
}

// validateCAName allows either a known acme.sh CA shortname or an https URL.
func validateCAName(ca string) error {
	switch ca {
	case "letsencrypt", "letsencrypt_test", "zerossl", "buypass", "buypass_test", "google", "sslcom":
		return nil
	}
	if strings.HasPrefix(ca, "https://") && !strings.ContainsAny(ca, " \t\n\r;&|`$\"'") {
		return nil
	}
	return fmt.Errorf("unsupported CA %q (use a known shortname or an https:// directory URL)", ca)
}
