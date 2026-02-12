// smtptunnel-client: SMTP Tunnel Client with SOCKS5 proxy, port forwarding, and diagnostics.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"smtptunnel/internal/config"
	"smtptunnel/internal/debug"
	"smtptunnel/internal/forward"
	"smtptunnel/internal/service"
	"smtptunnel/internal/socks5"
	"smtptunnel/internal/tunnel"
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
	case "ping":
		cmdPing()
	case "status":
		cmdStatus()
	case "check-config":
		cmdCheckConfig()
	case "install":
		cmdInstall()
	case "wizard":
		cmdWizard()
	case "service":
		cmdService()
	case "version":
		fmt.Printf("smtptunnel-client %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`smtptunnel-client %s — SMTP Tunnel Client

Usage:
  smtptunnel-client <command> [options]

Commands:
  run              Connect and start SOCKS/forward proxies
  ping             Test latency to server
  status           Connection diagnostics
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

func buildTLSConfig(cfg *config.Config) *tls.Config {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.Client.InsecureSkipVerify {
		tlsCfg.InsecureSkipVerify = true
	} else if cfg.Client.CACert != "" {
		caCert, err := os.ReadFile(cfg.Client.CACert)
		if err != nil {
			// Fall back to insecure if CA cert not found
			tlsCfg.InsecureSkipVerify = true
		} else {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caCert)
			tlsCfg.RootCAs = pool
		}
	} else {
		tlsCfg.InsecureSkipVerify = true
	}

	return tlsCfg
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
	if err := cfg.Validate("client"); err != nil {
		logger.Fatalf("Config error: %v", err)
	}

	tlsCfg := buildTLSConfig(cfg)

	reconnectDelay := cfg.Client.ReconnectDelay.Duration
	maxReconnectDelay := cfg.Client.MaxReconnectDelay.Duration
	if reconnectDelay == 0 {
		reconnectDelay = 2 * time.Second
	}
	if maxReconnectDelay == 0 {
		maxReconnectDelay = 30 * time.Second
	}

	logger.Printf("SMTP Tunnel Client %s starting", version)

	currentDelay := reconnectDelay

	for {
		client := tunnel.NewClient(cfg, tlsCfg, logger)

		if err := client.Connect(); err != nil {
			logger.Printf("Connection failed: %v, retrying in %v...", err, currentDelay)
			time.Sleep(currentDelay)
			currentDelay *= 2
			if currentDelay > maxReconnectDelay {
				currentDelay = maxReconnectDelay
			}
			continue
		}

		// Connected - reset delay
		currentDelay = reconnectDelay

		// Start receiver
		done := make(chan struct{})
		go func() {
			client.RunReceiver()
			close(done)
		}()

		// Start SOCKS5 servers
		var socksServers []*socks5.Server
		var forwarders []*forward.Forwarder
		var wg sync.WaitGroup

		for _, s := range cfg.Client.Socks {
			srv := &socks5.Server{
				ListenAddr: s.Listen,
				Username:   s.Username,
				Password:   s.Password,
				Tunnel:     client,
				Logger:     logger,
			}
			socksServers = append(socksServers, srv)

			wg.Add(1)
			go func(srv *socks5.Server) {
				defer wg.Done()
				if err := srv.ListenAndServe(); err != nil {
					logger.Printf("SOCKS error: %v", err)
				}
			}(srv)
		}

		// Start forwarders
		for _, f := range cfg.Client.Forward {
			proto := f.Protocol
			if proto == "" {
				proto = "tcp"
			}
			fwd := &forward.Forwarder{
				ListenAddr:  f.Listen,
				ForwardAddr: f.Forward,
				Protocol:    proto,
				Tunnel:      client,
				Logger:      logger,
			}
			forwarders = append(forwarders, fwd)

			wg.Add(1)
			go func(fwd *forward.Forwarder) {
				defer wg.Done()
				if err := fwd.ListenAndServe(); err != nil {
					logger.Printf("Forward error: %v", err)
				}
			}(fwd)
		}

		// Wait for connection to drop
		<-done

		// Close SOCKS servers
		for _, srv := range socksServers {
			srv.Close()
		}

		// Close forwarders
		for _, fwd := range forwarders {
			fwd.Close()
		}

		client.Disconnect()
		logger.Printf("Connection lost, reconnecting...")
	}
}

func cmdPing() {
	fs := flag.NewFlagSet("ping", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	count := fs.Int("n", 4, "Number of pings")
	fs.Parse(os.Args[1:])

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tlsCfg := buildTLSConfig(cfg)

	results, err := debug.Ping(cfg, tlsCfg, *count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(debug.FormatPingResults(cfg.Client.Server, results))
}

func cmdStatus() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	fs.Parse(os.Args[1:])

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	tlsCfg := buildTLSConfig(cfg)
	fmt.Print(debug.Status(cfg, tlsCfg))
}

func cmdCheckConfig() {
	fs := flag.NewFlagSet("check-config", flag.ExitOnError)
	configPath := fs.String("c", "config.toml", "Config file path")
	fs.Parse(os.Args[1:])

	fmt.Print(debug.CheckConfig(*configPath, "client"))
}

func cmdInstall() {
	fmt.Println("Installing smtptunnel-client...")

	if err := service.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created /etc/smtptunnel, /etc/smtptunnel/configs, /etc/smtptunnel/certs")

	if err := service.InstallBinary("smtptunnel-client"); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing binary: %v\n", err)
		fmt.Println("You may need to run as root.")
		os.Exit(1)
	}

	fmt.Println("Installation complete!")
}

func cmdWizard() {
	if err := service.RunClientWizard(); err != nil {
		fmt.Fprintf(os.Stderr, "Wizard error: %v\n", err)
		os.Exit(1)
	}
}

func cmdService() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service <install|list|remove|logs|stop|restart> [args]")
		os.Exit(1)
	}

	subcmd := os.Args[1]
	os.Args = append(os.Args[:1], os.Args[2:]...)

	switch subcmd {
	case "install":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service install <config.toml>")
			os.Exit(1)
		}
		if err := service.Install(os.Args[1], "client"); err != nil {
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
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service remove <name>")
			os.Exit(1)
		}
		if err := service.Remove(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "logs":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service logs <name> [-n lines]")
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
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service stop <name>")
			os.Exit(1)
		}
		if err := service.Stop(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "restart":
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: smtptunnel-client service restart <name>")
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
