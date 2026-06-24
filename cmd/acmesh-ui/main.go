// Command acmesh-ui is a lightweight WebUI for managing acme.sh on Linux
// servers. It ships as a single binary with the frontend embedded.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/server"
	"github.com/bright-color/acmesh-ui/internal/updater"
	"github.com/bright-color/acmesh-ui/internal/version"
)

const defaultConfigPath = "/etc/acmesh-ui/config.yaml"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		os.Exit(cmdServe(args))
	case "init":
		os.Exit(cmdInit(args))
	case "config":
		os.Exit(cmdConfig(args))
	case "scan":
		os.Exit(cmdScan(args))
	case "update":
		os.Exit(cmdUpdate(args))
	case "version", "--version", "-v":
		fmt.Println("acmesh-ui " + version.String())
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`acmesh-ui - WebUI for acme.sh

Usage:
  acmesh-ui serve        [--config PATH]   Start the web server
  acmesh-ui init         [--config PATH] [--force] [--bind IP] [--port N]
                                           Write config.yaml (prompts for bind/port)
  acmesh-ui config check [--config PATH]   Validate the configuration
  acmesh-ui scan         [--config PATH]   Scan certificates and print a summary
  acmesh-ui update       [--check]         Self-update to the latest release
  acmesh-ui version                        Print version information
`)
}

func configFlag(fs *flag.FlagSet) *string {
	return fs.String("config", defaultConfigPath, "path to config.yaml")
}

func cmdServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := configFlag(fs)
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	app, err := server.New(cfg, *cfgPath, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		return 1
	}
	return 0
}

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	cfgPath := configFlag(fs)
	force := fs.Bool("force", false, "overwrite an existing config file")
	bindFlag := fs.String("bind", "", "listen interface (skips the prompt)")
	portFlag := fs.Int("port", 0, "listen port (skips the prompt)")
	_ = fs.Parse(args)

	if _, err := os.Stat(*cfgPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "config %s already exists (use --force to overwrite)\n", *cfgPath)
		return 1
	}

	cfg := config.Default()

	// Determine bind/port: flags win; otherwise prompt interactively; otherwise
	// fall back to the defaults (non-interactive stdin, e.g. piped input).
	in := bufio.NewReader(os.Stdin)
	interactive := isInteractive()

	switch {
	case *bindFlag != "":
		cfg.Server.Bind = *bindFlag
	case interactive:
		fmt.Println("Configuring acmesh-ui. Press Enter to accept the default in [brackets].")
		cfg.Server.Bind = prompt(in, "Listen interface (IP to bind, 127.0.0.1 = loopback only)", cfg.Server.Bind)
	}

	switch {
	case *portFlag != 0:
		cfg.Server.Port = *portFlag
	case interactive:
		cfg.Server.Port = promptPort(in, "Listen port", cfg.Server.Port)
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port %d (must be 1-65535)\n", cfg.Server.Port)
		return 1
	}

	// If the chosen bind is reachable from the network and auth is off, the
	// server would refuse to start. Offer to acknowledge it now.
	if cfg.IsOpenBind() && cfg.AuthDisabled() {
		fmt.Printf("\nNote: bind=%s is reachable from the network and auth.mode=none.\n", cfg.Server.Bind)
		fmt.Println("acmesh-ui will refuse to start unless this is explicitly acknowledged.")
		if interactive && yesNo(in, "Set security.allow_open_without_auth=true now?", false) {
			cfg.Security.AllowOpenWithoutAuth = true
		} else {
			fmt.Println("Leaving it disabled - restrict access via VPN/SSH tunnel/reverse proxy, or set it later in the config.")
		}
	}

	if dir := dirOf(*cfgPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "create config dir: %v\n", err)
			return 1
		}
	}
	data, err := config.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal config: %v\n", err)
		return 1
	}
	header := "# acmesh-ui configuration\n# See docs/install.md for details.\n\n"
	if err := os.WriteFile(*cfgPath, append([]byte(header), data...), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write config: %v\n", err)
		return 1
	}
	fmt.Printf("\nwrote config to %s (bind=%s:%d)\n", *cfgPath, cfg.Server.Bind, cfg.Server.Port)
	fmt.Println("Next: review acme.binary / acme.home, then 'acmesh-ui config check' and 'acmesh-ui serve'.")
	return 0
}

// isInteractive reports whether stdin is a terminal (so prompting won't hang on
// piped or redirected input).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// prompt asks question, showing def as the default, and returns the entered
// value (or def if empty).
func prompt(r *bufio.Reader, question, def string) string {
	fmt.Printf("%s [%s]: ", question, def)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// promptPort prompts for a port, re-asking on invalid input.
func promptPort(r *bufio.Reader, question string, def int) int {
	for {
		s := prompt(r, question, strconv.Itoa(def))
		n, err := strconv.Atoi(s)
		if err == nil && n >= 1 && n <= 65535 {
			return n
		}
		fmt.Println("  please enter a number between 1 and 65535")
	}
}

// yesNo prompts a yes/no question with the given default.
func yesNo(r *bufio.Reader, question string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fmt.Printf("%s [%s]: ", question, hint)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	switch line {
	case "":
		return def
	case "y", "yes":
		return true
	default:
		return false
	}
}

func cmdConfig(args []string) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(os.Stderr, "usage: acmesh-ui config check [--config PATH]")
		return 2
	}
	fs := flag.NewFlagSet("config check", flag.ExitOnError)
	cfgPath := configFlag(fs)
	_ = fs.Parse(args[1:])

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		return 1
	}
	fmt.Printf("OK: %s is valid\n", *cfgPath)
	fmt.Printf("  bind=%s:%d auth=%s acme=%s\n", cfg.Server.Bind, cfg.Server.Port, cfg.Auth.Mode, cfg.Acme.Binary)
	if cfg.AuthDisabled() {
		fmt.Println("  note: auth.mode=none - restrict access via VPN/SSH tunnel/reverse proxy")
	}
	return 0
}

func cmdScan(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	cfgPath := configFlag(fs)
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}
	scanner := acme.NewScanner(cfg.Acme.Home, cfg.Certs.ExpiringSoonDays)
	list, err := scanner.Scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		return 1
	}
	fmt.Printf("found %d certificate(s) in %s\n", len(list), cfg.Acme.Home)
	for _, c := range list {
		fmt.Printf("  %-40s %-10s %4dd  %s\n", c.MainDomain, c.Status, c.DaysRemaining, c.KeyType)
	}
	return 0
}

func cmdUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "only check for a newer version, do not install")
	_ = fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chk, err := updater.Check(ctx, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
		return 1
	}
	fmt.Printf("installed: %s\nlatest:    %s\n", chk.Current, chk.Latest)
	if !chk.UpdateAvailable {
		fmt.Println("already up to date.")
		return 0
	}
	if *checkOnly {
		fmt.Println("update available - run 'acmesh-ui update' to install.")
		return 0
	}
	if !chk.CanSelfUpdate {
		fmt.Fprintf(os.Stderr, "cannot replace the binary: %s\nTry: sudo acmesh-ui update\n", chk.Note)
		return 1
	}
	fmt.Printf("downloading %s %s ...\n", chk.Asset, chk.Latest)
	tag, err := updater.Apply(ctx, chk.Latest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		return 1
	}
	fmt.Printf("updated to %s. Restart the service to run it: systemctl restart acmesh-ui\n", tag)
	return 0
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return ""
}
