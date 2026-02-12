# 📧 SMTP Tunnel Proxy (Go Edition)

> **A high-speed covert tunnel that disguises TCP traffic as SMTP email communication to bypass Deep Packet Inspection (DPI) firewalls.**

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐      ┌──────────────┐
│ Application │─────▶│   Client    │─────▶│   Server    │─────▶│  Internet    │
│  (Browser)  │ TCP  │ SOCKS5:1080 │ SMTP │  Port 587   │ TCP  │              │
│             │◀─────│             │◀─────│             │◀─────│              │
└─────────────┘      └─────────────┘      └─────────────┘      └──────────────┘
                            │                    │
                            │   Looks like       │
                            │   Email Traffic    │
                            ▼                    ▼
                     ┌────────────────────────────────┐
                     │     DPI Firewall               │
                     │  ✅ Sees: Normal SMTP Session  │
                     │  ❌ Cannot see: Tunnel Data    │
                     └────────────────────────────────┘
```

---

## 🎯 Features

| Feature | Description |
|---------|-------------|
| 🔒 **TLS Encryption** | All traffic encrypted with TLS 1.2+ after SMTP STARTTLS |
| 🎭 **DPI Evasion** | Handshake mimics Postfix SMTP servers; supports traffic shaping |
| ⚡ **High Performance** | Native Go implementation with binary multiplexing protocol |
| 👥 **User Management** | Built-in CLI for managing users, secrets, and whitelists |
| 🔑 **Secure Auth** | HMAC-SHA256 authenticated tokens with anti-replay protection |
| 🌐 **SOCKS5 Proxy** | Multiple local SOCKS5 listeners per client |
| 🛡️ **Stealth Mode** | Configurable random delays, packet padding, and dummy traffic |
| 📦 **Service Ready** | Built-in systemd service installation for Linux |

---

## 🚀 Quick Start

### 1. Build from Source
Ensure you have Go 1.18+ installed.

```bash
git clone https://github.com/youruser/smtptunnel.git
cd smtptunnel
make build
```
This produces `smtptunnel-server` and `smtptunnel-client`.

### 2. Server Setup (VPS)
1. **Generate Certificates**:
   ```bash
   ./smtptunnel-server gencerts -hostname yourdomain.com
   ```
2. **Add a User**:
   ```bash
   ./smtptunnel-server adduser -c config.toml alice
   ```
3. **Start Server**:
   ```bash
   ./smtptunnel-server run -c config.toml
   ```

### 3. Client Setup (Local)
1. Copy `config.toml`, `ca.crt`, and `smtptunnel-client` to your machine.
2. Edit `config.toml` to include your server IP and credentials.
3. **Connect**:
   ```bash
   ./smtptunnel-client run -c config.toml
   ```

---

## ⚙️ Configuration (`config.toml`)

The project uses a unified TOML configuration for both server and client.

### [server]
- `listen`: Interface and port to listen on (default: `0.0.0.0:587`).
- `hostname`: SMTP banner hostname (must match SSL certificate).
- `cert_file`/`key_file`: Path to TLS credentials.
- `users_file`: (Optional) External file for user database.

### [client]
- `server`: Remote server address (`host:port`).
- `username`/`secret`: Authentication credentials.
- `ca_cert`: Path to CA certificate for verification.
- `insecure_skip_verify`: Set `true` to skip certificate validation (not recommended).
- `reconnect_delay`: Initial retry delay (e.g., `2s`).

### [[client.socks]]
Define multiple SOCKS5 listeners:
```toml
[[client.socks]]
  listen = "127.0.0.1:1080"
  username = "user" # Optional SOCKS5 auth
  password = "pass"
```

### [stealth]
Advanced DPI evasion settings:
- `enabled`: Enable traffic shaping and padding.
- `min_delay_ms`/`max_delay_ms`: Random delay range between packet groups.
- `padding_sizes`: List of target packet sizes for padding (e.g., `[1024, 2048, 4096]`).
- `dummy_probability`: Chance (0.0 - 1.0) of sending a dummy packet during idle.

---

## 🔧 Command Reference

### Server Subcommands
- `run`: Start the server.
- `adduser <name>`: Add a new user with an auto-generated secret.
- `deluser <name>`: Remove a user.
- `listusers [-v]`: List all users and their secrets.
- `gencerts`: Generate CA and server TLS certificates.
- `check-config`: Validate the TOML configuration.
- `install-service`: Install a systemd unit file for the server.

### Client Subcommands
- `run`: Connect to the server and start SOCKS5 proxies.
- `ping`: Measure RTT latency through the tunnel.
- `status`: Check connection and diagnostic information.
- `install-service`: Install a systemd unit file for the client.

---

## 📖 Usage Guide

Once the client is running, configure your application (Browser, CLI, etc.) to use the SOCKS5 proxy at `127.0.0.1:1080`.

**Example (curl):**
```bash
curl -x socks5h://127.0.0.1:1080 https://ifconfig.me
```

---

## 🛡️ Technical Overview

### SMTP Handshake
The tunnel establishes a legitimate-looking SMTP session:
1. Server sends `220 ESMTP` banner.
2. Client sends `EHLO`.
3. Server offers `STARTTLS`.
4. TLS Handshake completed.
5. Client authenticates via a custom `AUTH PLAIN` token.
6. Communication switches to `BINARY` mode for data streaming.

### Binary Protocol
After the handshake, the tunnel uses a layered protocol:
- **Framing**: Type(1), Channel(2), Length(2), Payload(N).
- **Multiplexing**: Supports multiple simultaneous TCP streams over a single SMTP connection.
- **Crypto**: ChaCha20-Poly1305 encryption on top of TLS for extra defense-in-depth.

---

## 📄 License
Educational and authorized use only. Use responsibly.
