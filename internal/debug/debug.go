// Package debug provides diagnostic tools for the tunnel.
package debug

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"smtptunnel/internal/config"
	"smtptunnel/internal/smtp"
	"smtptunnel/internal/tunnel"
)

// PingResult stores a single ping measurement.
type PingResult struct {
	Seq int
	RTT time.Duration
	Err error
}

// Ping connects to the server and measures round-trip time.
func Ping(cfg *config.Config, tlsCfg *tls.Config, count int) ([]PingResult, error) {
	if count <= 0 {
		count = 4
	}

	// Connect
	rawConn, err := net.DialTimeout("tcp", cfg.Client.Server, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	serverHost := smtp.HostFromAddr(cfg.Client.Server)
	tlsConn, err := smtp.ClientHandshake(rawConn, serverHost, cfg.Client.Username, cfg.Client.Secret, tlsCfg)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	client := tunnel.NewClient(cfg, tlsCfg, nil)
	// Inject the already-established connection
	client.InjectConn(tlsConn)

	go client.RunReceiver()
	defer client.Disconnect()

	results := make([]PingResult, count)
	for i := 0; i < count; i++ {
		rtt, err := client.Ping()
		results[i] = PingResult{Seq: i + 1, RTT: rtt, Err: err}
		if i < count-1 {
			time.Sleep(time.Second)
		}
	}

	return results, nil
}

// FormatPingResults formats ping results for display.
func FormatPingResults(server string, results []PingResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PING %s (%d pings):\n", server, len(results)))

	var totalRTT time.Duration
	var minRTT, maxRTT time.Duration
	successCount := 0

	for _, r := range results {
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("  seq=%d error: %v\n", r.Seq, r.Err))
			continue
		}
		sb.WriteString(fmt.Sprintf("  seq=%d rtt=%v\n", r.Seq, r.RTT.Round(time.Microsecond)))
		totalRTT += r.RTT
		successCount++
		if minRTT == 0 || r.RTT < minRTT {
			minRTT = r.RTT
		}
		if r.RTT > maxRTT {
			maxRTT = r.RTT
		}
	}

	sb.WriteString(fmt.Sprintf("\n--- %s ping statistics ---\n", server))
	sb.WriteString(fmt.Sprintf("%d packets transmitted, %d received, %.0f%% loss\n",
		len(results), successCount,
		float64(len(results)-successCount)/float64(len(results))*100))
	if successCount > 0 {
		avg := totalRTT / time.Duration(successCount)
		sb.WriteString(fmt.Sprintf("rtt min/avg/max = %v/%v/%v\n",
			minRTT.Round(time.Microsecond),
			avg.Round(time.Microsecond),
			maxRTT.Round(time.Microsecond)))
	}

	return sb.String()
}

// Status checks connectivity to the server.
func Status(cfg *config.Config, tlsCfg *tls.Config) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Server: %s\n", cfg.Client.Server))
	sb.WriteString(fmt.Sprintf("Username: %s\n", cfg.Client.Username))
	sb.WriteString(fmt.Sprintf("SOCKS proxies: %d\n", len(cfg.Client.Socks)))
	for _, s := range cfg.Client.Socks {
		auth := "none"
		if s.Username != "" {
			auth = fmt.Sprintf("user/pass (%s)", s.Username)
		}
		sb.WriteString(fmt.Sprintf("  %s (auth: %s)\n", s.Listen, auth))
	}

	// Test TCP connection
	sb.WriteString("\nConnectivity:\n")

	start := time.Now()
	conn, err := net.DialTimeout("tcp", cfg.Client.Server, 10*time.Second)
	if err != nil {
		sb.WriteString(fmt.Sprintf("  TCP: FAIL (%v)\n", err))
		return sb.String()
	}
	tcpTime := time.Since(start)
	sb.WriteString(fmt.Sprintf("  TCP: OK (%v)\n", tcpTime.Round(time.Microsecond)))

	// Test TLS + SMTP handshake
	start = time.Now()
	serverHost := smtp.HostFromAddr(cfg.Client.Server)
	tlsConn, err := smtp.ClientHandshake(conn, serverHost, cfg.Client.Username, cfg.Client.Secret, tlsCfg)
	if err != nil {
		sb.WriteString(fmt.Sprintf("  Handshake: FAIL (%v)\n", err))
		conn.Close()
		return sb.String()
	}
	hsTime := time.Since(start)
	sb.WriteString(fmt.Sprintf("  Handshake: OK (%v)\n", hsTime.Round(time.Microsecond)))
	sb.WriteString("  Auth: OK\n")
	sb.WriteString("  Binary mode: OK\n")

	tlsConn.Close()

	return sb.String()
}

// CheckConfig validates a config file.
func CheckConfig(path, mode string) string {
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Sprintf("ERROR: %v\n", err)
	}

	if err := cfg.Validate(mode); err != nil {
		return fmt.Sprintf("INVALID: %v\n", err)
	}

	var sb strings.Builder
	sb.WriteString("Config OK\n")

	if mode == "server" || mode == "" {
		sb.WriteString(fmt.Sprintf("  Server listen: %s\n", cfg.Server.Listen))
		sb.WriteString(fmt.Sprintf("  Server hostname: %s\n", cfg.Server.Hostname))
		sb.WriteString(fmt.Sprintf("  Users: %d\n", len(cfg.Server.Users)))
	}
	if mode == "client" || mode == "" {
		sb.WriteString(fmt.Sprintf("  Client server: %s\n", cfg.Client.Server))
		sb.WriteString(fmt.Sprintf("  SOCKS proxies: %d\n", len(cfg.Client.Socks)))
	}
	sb.WriteString(fmt.Sprintf("  Stealth: %v\n", cfg.Stealth.Enabled))

	return sb.String()
}
