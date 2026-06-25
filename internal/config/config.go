// Package config defines the acmesh-ui configuration model and loading rules.
package config

// Config is the full on-disk configuration (config.yaml).
type Config struct {
	Server   Server          `yaml:"server"`
	Auth     Auth            `yaml:"auth"`
	Security Security        `yaml:"security"`
	Acme     Acme            `yaml:"acme"`
	Jobs     Jobs            `yaml:"jobs"`
	UI       UI              `yaml:"ui"`
	Reloads  []ReloadCommand `yaml:"reload_commands"`
	Data     Data            `yaml:"data"`
	Certs    Certs           `yaml:"certs"`
}

// Server holds bind/listen settings.
type Server struct {
	Bind      string `yaml:"bind" json:"bind"`
	Port      int    `yaml:"port" json:"port"`
	PublicURL string `yaml:"public_url" json:"public_url"`
}

// Auth selects the authentication mode. Version 1 fully supports "none".
type Auth struct {
	Mode string `yaml:"mode" json:"mode"` // none | basic | reverse_proxy
}

// Security gates open binding and points at the secret key file.
type Security struct {
	AllowOpenWithoutAuth bool   `yaml:"allow_open_without_auth"`
	SecretKeyFile        string `yaml:"secret_key_file"`
	// AllowFreeReloadCmd, when true, permits operator-defined reload commands
	// beyond the configured templates (still never a shell string).
	AllowFreeReloadCmd bool `yaml:"allow_free_reloadcmd"`
	// CertBasePaths optionally restricts --install-cert target paths.
	CertBasePaths []string `yaml:"cert_base_paths"`
	// WebrootBasePaths optionally restricts HTTP-01 webroot paths.
	WebrootBasePaths []string `yaml:"webroot_base_paths"`
}

// Acme points at the external acme.sh installation.
type Acme struct {
	Binary         string `yaml:"binary" json:"binary"`
	Home           string `yaml:"home" json:"home"`
	DefaultCA      string `yaml:"default_ca" json:"default_ca"`
	DefaultKeyType string `yaml:"default_key_type" json:"default_key_type"`
	DefaultWebroot string `yaml:"default_webroot" json:"default_webroot"`
}

// Jobs configures the background job runner.
type Jobs struct {
	MaxParallel      int `yaml:"max_parallel" json:"max_parallel"`
	LogRetentionDays int `yaml:"log_retention_days" json:"log_retention_days"`
	// TimeoutSeconds bounds a single acme.sh invocation. 0 => default.
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
}

// UI configures cosmetic UI behaviour.
type UI struct {
	Title           string `yaml:"title" json:"title"`
	ShowAuthWarning bool   `yaml:"show_auth_warning" json:"show_auth_warning"`
}

// ReloadCommand is an allow-listed reload template offered for --install-cert.
type ReloadCommand struct {
	Name    string   `yaml:"name" json:"name"`
	Command []string `yaml:"command" json:"command"`
}

// Data is where acmesh-ui keeps its own state.
type Data struct {
	Dir string `yaml:"dir" json:"dir"`
}

// Certs tunes certificate expiry evaluation.
type Certs struct {
	ExpiringSoonDays int `yaml:"expiring_soon_days" json:"expiring_soon_days"`
}

// Default returns a Config populated with safe defaults.
func Default() Config {
	return Config{
		Server: Server{Bind: "127.0.0.1", Port: 8090},
		Auth:   Auth{Mode: "none"},
		Security: Security{
			AllowOpenWithoutAuth: false,
			SecretKeyFile:        "/etc/acmesh-ui/secret.key",
		},
		Acme: Acme{
			Binary:         "/root/.acme.sh/acme.sh",
			Home:           "/root/.acme.sh",
			DefaultKeyType: "ec-256",
		},
		Jobs:  Jobs{MaxParallel: 2, LogRetentionDays: 30, TimeoutSeconds: 1800},
		UI:    UI{Title: "acmesh-ui", ShowAuthWarning: true},
		Data:  Data{Dir: "/var/lib/acmesh-ui"},
		Certs: Certs{ExpiringSoonDays: 30},
		Reloads: []ReloadCommand{
			{Name: "Reload nginx", Command: []string{"systemctl", "reload", "nginx"}},
			{Name: "Reload Apache", Command: []string{"systemctl", "reload", "apache2"}},
			{Name: "Reload HAProxy", Command: []string{"systemctl", "reload", "haproxy"}},
			{Name: "Restart Postfix", Command: []string{"systemctl", "restart", "postfix"}},
			{Name: "Restart Dovecot", Command: []string{"systemctl", "restart", "dovecot"}},
			{Name: "Reload Caddy", Command: []string{"systemctl", "reload", "caddy"}},
		},
	}
}

// IsOpenBind reports whether the bind address is non-loopback (reachable from
// other hosts).
func (c Config) IsOpenBind() bool {
	switch c.Server.Bind {
	case "127.0.0.1", "::1", "localhost", "":
		return false
	default:
		return true
	}
}

// AuthDisabled reports whether no internal authentication is enforced.
func (c Config) AuthDisabled() bool {
	return c.Auth.Mode == "" || c.Auth.Mode == "none"
}
