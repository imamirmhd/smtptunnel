// smtptunnel-server: SMTP Tunnel Server with user management and cert generation.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"

	"smtptunnel/internal/certs"
	"smtptunnel/internal/config"
	"smtptunnel/internal/debug"
	"smtptunnel/internal/tunnel"
	"smtptunnel/internal/users"
)

const version = "2.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch cmd {
	case "run":
		cmdRun()
	case "adduser":
		cmdAddUser()
	case "deluser":
		cmdDelUser()
	case "listusers":
		cmdListUsers()
	case "gencerts":
		cmdGenCerts()
	case "check-config":
		cmdCheckConfig()
	case "install-service":
		cmdInstallService()
	case "version":
		fmt.Printf("smtptunnel-server %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`smtptunnel-server %s — SMTP Tunnel Server

Usage:
  smtptunnel-server <command> [options]

Commands:
  run              Start the tunnel server
  adduser          Add a new user to the config
  deluser          Remove a user from the config
  listusers        List all configured users
  gencerts         Generate TLS certificates
  check-config     Validate configuration file
  install-service  Install systemd service
  version          Show version

`, version)
}

func cmdRun() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	debugMode := fs.Bool("debug", false, "Enable debug logging")
	fs.Parse(os.Args[1:])

	logger := log.New(os.Stdout, "", log.LstdFlags)
	if *debugMode {
		logger.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("Load config: %v", err)
	}
	if err := cfg.Validate("server"); err != nil {
		logger.Fatalf("Config error: %v", err)
	}

	// Load TLS
	cert, err := tls.LoadX509KeyPair(cfg.Server.CertFile, cfg.Server.KeyFile)
	if err != nil {
		logger.Fatalf("Load TLS: %v", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	srv := tunnel.NewServer(cfg, tlsCfg, logger)
	logger.Printf("SMTP Tunnel Server %s starting", version)
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("Server error: %v", err)
	}
}

func cmdAddUser() {
	fs := flag.NewFlagSet("adduser", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	secret := fs.String("secret", "", "Secret (auto-generated if empty)")
	noLogging := fs.Bool("no-logging", false, "Disable logging for this user")
	fs.Parse(os.Args[1:])

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server adduser <username> [-c config.toml] [--secret <s>]")
		os.Exit(1)
	}
	username := fs.Arg(0)

	logging := !*noLogging
	if err := users.AddUser(*configPath, username, *secret, nil, logging); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User '%s' added to %s\n", username, *configPath)
	fmt.Println("Restart the server to apply changes.")
}

func cmdDelUser() {
	fs := flag.NewFlagSet("deluser", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	fs.Parse(os.Args[1:])

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server deluser <username> [-c config.toml]")
		os.Exit(1)
	}
	username := fs.Arg(0)

	if err := users.DelUser(*configPath, username); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User '%s' removed from %s\n", username, *configPath)
}

func cmdListUsers() {
	fs := flag.NewFlagSet("listusers", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	verbose := fs.Bool("v", false, "Verbose output")
	fs.Parse(os.Args[1:])

	output, err := users.ListUsers(*configPath, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(output)
}

func cmdGenCerts() {
	fs := flag.NewFlagSet("gencerts", flag.ExitOnError)
	hostname := fs.String("hostname", "mail.example.com", "Server hostname")
	outputDir := fs.String("output-dir", ".", "Output directory")
	days := fs.Int("days", 1095, "Certificate validity in days")
	keySize := fs.Int("key-size", 2048, "RSA key size")
	fs.Parse(os.Args[1:])

	fmt.Printf("Generating certificates for %s...\n", *hostname)
	if err := certs.Generate(certs.Options{
		Hostname:  *hostname,
		OutputDir: *outputDir,
		Days:      *days,
		KeySize:   *keySize,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nCertificate generation complete!")
}

func cmdCheckConfig() {
	fs := flag.NewFlagSet("check-config", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	fs.Parse(os.Args[1:])

	fmt.Print(debug.CheckConfig(*configPath, "server"))
}

func cmdInstallService() {
	service := `[Unit]
Description=SMTP Tunnel Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/smtptunnel-server run -c /etc/smtptunnel/config.toml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
	path := "/etc/systemd/system/smtptunnel-server.service"
	if err := os.WriteFile(path, []byte(service), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing service file: %v\n", err)
		fmt.Println("You may need to run as root.")
		os.Exit(1)
	}
	fmt.Printf("Service file written to %s\n", path)
	fmt.Println("Run: systemctl daemon-reload && systemctl enable --now smtptunnel-server")
}
