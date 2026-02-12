// Package smtp implements the SMTP handshake protocol used as a cover.
package smtp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"smtptunnel/internal/crypto"
)

const readTimeout = 60 * time.Second

// ServerHandshake performs the server-side SMTP handshake over a raw TCP connection.
// Returns the authenticated username, the upgraded TLS connection, or an error.
func ServerHandshake(conn net.Conn, hostname string, tlsConfig *tls.Config, users map[string]string) (string, net.Conn, error) {
	r := bufio.NewReader(conn)

	writeLine := func(line string) error {
		conn.SetWriteDeadline(time.Now().Add(readTimeout))
		_, err := fmt.Fprintf(conn, "%s\r\n", line)
		return err
	}

	readLine := func() (string, error) {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	// 220 greeting
	if err := writeLine(fmt.Sprintf("220 %s ESMTP Postfix (Ubuntu)", hostname)); err != nil {
		return "", nil, err
	}

	// EHLO
	line, err := readLine()
	if err != nil {
		return "", nil, err
	}
	upper := strings.ToUpper(line)
	if !strings.HasPrefix(upper, "EHLO") && !strings.HasPrefix(upper, "HELO") {
		return "", nil, fmt.Errorf("expected EHLO, got: %s", line)
	}

	// Capabilities
	writeLine(fmt.Sprintf("250-%s", hostname))
	writeLine("250-STARTTLS")
	writeLine("250-AUTH PLAIN LOGIN")
	writeLine("250 8BITMIME")

	// STARTTLS
	line, err = readLine()
	if err != nil {
		return "", nil, err
	}
	if strings.ToUpper(line) != "STARTTLS" {
		return "", nil, fmt.Errorf("expected STARTTLS, got: %s", line)
	}
	writeLine("220 2.0.0 Ready to start TLS")

	// Upgrade to TLS
	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return "", nil, fmt.Errorf("tls handshake: %w", err)
	}

	// Re-wrap with buffered reader on TLS conn
	conn = tlsConn
	r = bufio.NewReader(conn)

	// Reinitialize closures to use TLS conn
	writeLine = func(line string) error {
		conn.SetWriteDeadline(time.Now().Add(readTimeout))
		_, err := fmt.Fprintf(conn, "%s\r\n", line)
		return err
	}
	readLine = func() (string, error) {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	// EHLO again
	line, err = readLine()
	if err != nil {
		return "", nil, err
	}
	upper = strings.ToUpper(line)
	if !strings.HasPrefix(upper, "EHLO") && !strings.HasPrefix(upper, "HELO") {
		return "", nil, fmt.Errorf("expected EHLO after TLS, got: %s", line)
	}

	writeLine(fmt.Sprintf("250-%s", hostname))
	writeLine("250-AUTH PLAIN LOGIN")
	writeLine("250 8BITMIME")

	// AUTH
	line, err = readLine()
	if err != nil {
		return "", nil, err
	}
	if !strings.HasPrefix(strings.ToUpper(line), "AUTH") {
		return "", nil, fmt.Errorf("expected AUTH, got: %s", line)
	}

	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		writeLine("535 5.7.8 Authentication failed")
		return "", nil, fmt.Errorf("malformed AUTH")
	}

	token := parts[2]
	valid, username := crypto.VerifyAuthToken(token, users, 300)
	if !valid {
		writeLine("535 5.7.8 Authentication failed")
		return "", nil, fmt.Errorf("auth failed for token")
	}

	writeLine("235 2.7.0 Authentication successful")

	// BINARY mode signal
	line, err = readLine()
	if err != nil {
		return "", nil, err
	}
	if line != "BINARY" {
		return "", nil, fmt.Errorf("expected BINARY, got: %s", line)
	}
	writeLine("299 Binary mode activated")

	// Clear deadlines for streaming
	conn.SetDeadline(time.Time{})

	return username, conn, nil
}

// ClientHandshake performs the client-side SMTP handshake.
// Returns the upgraded TLS connection or an error.
func ClientHandshake(conn net.Conn, serverHost, username, secret string, tlsConfig *tls.Config) (net.Conn, error) {
	r := bufio.NewReader(conn)

	writeLine := func(line string) error {
		conn.SetWriteDeadline(time.Now().Add(readTimeout))
		_, err := fmt.Fprintf(conn, "%s\r\n", line)
		return err
	}

	readLine := func() (string, error) {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	expect := func(prefix string) error {
		line, err := readLine()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, prefix) {
			return fmt.Errorf("expected %s, got: %s", prefix, line)
		}
		return nil
	}

	expectMultiline := func() error {
		for {
			line, err := readLine()
			if err != nil {
				return err
			}
			if strings.HasPrefix(line, "250 ") {
				return nil
			}
			if strings.HasPrefix(line, "250-") {
				continue
			}
			return fmt.Errorf("unexpected response: %s", line)
		}
	}

	// 220 greeting
	if err := expect("220"); err != nil {
		return nil, fmt.Errorf("greeting: %w", err)
	}

	// EHLO
	if err := writeLine("EHLO tunnel-client.local"); err != nil {
		return nil, err
	}
	if err := expectMultiline(); err != nil {
		return nil, fmt.Errorf("ehlo: %w", err)
	}

	// STARTTLS
	if err := writeLine("STARTTLS"); err != nil {
		return nil, err
	}
	if err := expect("220"); err != nil {
		return nil, fmt.Errorf("starttls: %w", err)
	}

	// Upgrade TLS
	cfg := tlsConfig.Clone()
	if cfg.ServerName == "" {
		cfg.ServerName = serverHost
	}
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("tls handshake: %w", err)
	}

	conn = tlsConn
	r = bufio.NewReader(conn)
	writeLine = func(line string) error {
		conn.SetWriteDeadline(time.Now().Add(readTimeout))
		_, err := fmt.Fprintf(conn, "%s\r\n", line)
		return err
	}
	readLine = func() (string, error) {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	expect = func(prefix string) error {
		line, err := readLine()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, prefix) {
			return fmt.Errorf("expected %s, got: %s", prefix, line)
		}
		return nil
	}
	expectMultiline = func() error {
		for {
			line, err := readLine()
			if err != nil {
				return err
			}
			if strings.HasPrefix(line, "250 ") {
				return nil
			}
			if strings.HasPrefix(line, "250-") {
				continue
			}
			return fmt.Errorf("unexpected response: %s", line)
		}
	}

	// EHLO again
	if err := writeLine("EHLO tunnel-client.local"); err != nil {
		return nil, err
	}
	if err := expectMultiline(); err != nil {
		return nil, fmt.Errorf("ehlo post-tls: %w", err)
	}

	// AUTH
	timestamp := time.Now().Unix()
	token := crypto.GenerateAuthToken(secret, username, timestamp)
	if err := writeLine(fmt.Sprintf("AUTH PLAIN %s", token)); err != nil {
		return nil, err
	}
	if err := expect("235"); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	// Switch to binary
	if err := writeLine("BINARY"); err != nil {
		return nil, err
	}
	if err := expect("299"); err != nil {
		return nil, fmt.Errorf("binary mode: %w", err)
	}

	// Clear deadlines for streaming
	conn.SetDeadline(time.Time{})

	return conn, nil
}

// HostFromAddr extracts just the hostname from a host:port address.
func HostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// Discard reads and discards from reader (used for cleanup).
func Discard(r io.Reader) {
	io.Copy(io.Discard, r)
}
