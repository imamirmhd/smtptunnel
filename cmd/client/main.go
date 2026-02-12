// smtptunnel-client: SMTP Tunnel Client with SOCKS5 proxy, ping, and diagnostics.
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
	"smtptunnel/internal/socks5"
	"smtptunnel/internal/tunnel"
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
	case "ping":
		cmdPing()
	case "status":
		cmdStatus()
	case "check-config":
		cmdCheckConfig()
	case "install-service":
		cmdInstallService()
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
  run              Connect and start SOCKS proxies
  ping             Test latency to server
  status           Connection diagnostics
  check-config     Validate configuration file
  install-service  Install systemd service
  version          Show version

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

		// Wait for connection to drop
		<-done

		// Close SOCKS servers
		for _, srv := range socksServers {
			srv.Close()
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

func cmdInstallService() {
	service := `[Unit]
Description=SMTP Tunnel Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/smtptunnel-client run -c /etc/smtptunnel/config.toml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
	path := "/etc/systemd/system/smtptunnel-client.service"
	if err := os.WriteFile(path, []byte(service), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing service file: %v\n", err)
		fmt.Println("You may need to run as root.")
		os.Exit(1)
	}
	fmt.Printf("Service file written to %s\n", path)
	fmt.Println("Run: systemctl daemon-reload && systemctl enable --now smtptunnel-client")
}
