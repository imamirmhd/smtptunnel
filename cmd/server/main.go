// smtptunnel-server: SMTP Tunnel Server with user management, cert generation, and service management.
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
	"smtptunnel/internal/service"
	"smtptunnel/internal/tunnel"
	"smtptunnel/internal/users"
)

const version = "2.1.0"

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
	case "install":
		cmdInstall()
	case "wizard":
		cmdWizard()
	case "service":
		cmdService()
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
  install          Install binary and create directories
  wizard           Interactive configuration generator
  service          Manage systemd services
  version          Show version

Service subcommands:
  service install <config.toml>   Register config as systemd service
  service list                    List registered services
  service remove <name>           Remove a service
  service logs <name> [-n lines]  View service logs
  service stop <name>             Stop a service
  service restart <name>          Restart a service

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
	outputDir := fs.String("output-dir", "", "Output directory (default: /etc/smtptunnel/certs/<hostname>)")
	days := fs.Int("days", 1095, "Certificate validity in days")
	keySize := fs.Int("key-size", 2048, "RSA key size")
	fs.Parse(os.Args[1:])

	if *outputDir == "" {
		*outputDir = fmt.Sprintf("/etc/smtptunnel/certs/%s", *hostname)
	}

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
	fmt.Printf("Files written to: %s\n", *outputDir)
}

func cmdCheckConfig() {
	fs := flag.NewFlagSet("check-config", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	fs.Parse(os.Args[1:])

	fmt.Print(debug.CheckConfig(*configPath, "server"))
}

func cmdInstall() {
	fmt.Println("Installing smtptunnel-server...")

	if err := service.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created /etc/smtptunnel, /etc/smtptunnel/configs, /etc/smtptunnel/certs")

	if err := service.InstallBinary("smtptunnel-server"); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing binary: %v\n", err)
		fmt.Println("You may need to run as root.")
		os.Exit(1)
	}

	fmt.Println("Installation complete!")
}

func cmdWizard() {
	if err := service.RunServerWizard(); err != nil {
		fmt.Fprintf(os.Stderr, "Wizard error: %v\n", err)
		os.Exit(1)
	}
}

func cmdService() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service <install|list|remove|logs|stop|restart> [args]")
		os.Exit(1)
	}

	subcmd := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch subcmd {
	case "install":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service install <config.toml>")
			os.Exit(1)
		}
		if err := service.Install(os.Args[1], "server"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := service.List(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "remove":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service remove <name>")
			os.Exit(1)
		}
		if err := service.Remove(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "logs":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service logs <name> [-n lines]")
			os.Exit(1)
		}
		name := os.Args[1]
		lines := 50
		fs := flag.NewFlagSet("logs", flag.ExitOnError)
		fs.IntVar(&lines, "n", 50, "Number of lines")
		fs.Parse(os.Args[2:])
		if err := service.Logs(name, lines); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "stop":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service stop <name>")
			os.Exit(1)
		}
		if err := service.Stop(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "restart":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-server service restart <name>")
			os.Exit(1)
		}
		if err := service.Restart(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown service command: %s\n", subcmd)
		os.Exit(1)
	}
}
