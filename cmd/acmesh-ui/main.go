// Command acmesh-ui is a lightweight WebUI for managing acme.sh on Linux
// servers. It ships as a single binary with the frontend embedded.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bright-color/acmesh-ui/internal/acme"
	"github.com/bright-color/acmesh-ui/internal/config"
	"github.com/bright-color/acmesh-ui/internal/server"
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
  acmesh-ui init         [--config PATH] [--force]   Write a sample config.yaml
  acmesh-ui config check [--config PATH]   Validate the configuration
  acmesh-ui scan         [--config PATH]   Scan certificates and print a summary
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
	_ = fs.Parse(args)

	if _, err := os.Stat(*cfgPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "config %s already exists (use --force to overwrite)\n", *cfgPath)
		return 1
	}
	if dir := dirOf(*cfgPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "create config dir: %v\n", err)
			return 1
		}
	}
	data, err := config.Marshal(config.Default())
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal config: %v\n", err)
		return 1
	}
	header := "# acmesh-ui configuration\n# See docs/install.md for details.\n\n"
	if err := os.WriteFile(*cfgPath, append([]byte(header), data...), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write config: %v\n", err)
		return 1
	}
	fmt.Printf("wrote sample config to %s\n", *cfgPath)
	return 0
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

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return ""
}
