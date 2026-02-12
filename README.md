# 📧 SMTP Tunnel

> **A high-speed covert tunnel that disguises TCP/UDP traffic as SMTP email communication to bypass Deep Packet Inspection (DPI) firewalls.**

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐      ┌──────────────┐
│ Application │─────▶│   Client    │─────▶│   Server    │─────▶│  Internet    │
│  (Browser)  │ TCP  │ SOCKS5:1080 │ SMTP │  Port 587   │ TCP  │              │
│             │◀─────│  FWD:8080   │◀─────│             │◀─────│              │
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
| 🔀 **Port Forwarding** | TCP/UDP forwarding for direct traffic redirection |
| 🛡️ **Stealth Mode** | Configurable random delays, packet padding, and dummy traffic |
| 📦 **Service Ready** | Built-in systemd service management |
| 🧙 **Setup Wizard** | Interactive configuration generator for quick setup |

---

## 🚀 Quick Start

### 1. Build & Install

```bash
git clone https://github.com/youruser/smtptunnel.git
cd smtptunnel
make build
```

This produces `smtptunnel-server` and `smtptunnel-client`.

**System-wide installation** (requires root):

```bash
# Via Makefile:
sudo make install

# Or via the binary itself:
sudo ./smtptunnel-server install
sudo ./smtptunnel-client install
```

The `install` command:
- Copies the binary to `/usr/local/bin/`
- Creates `/etc/smtptunnel/` (base configuration directory)
- Creates `/etc/smtptunnel/configs/` (per-instance configuration files)
- Creates `/etc/smtptunnel/certs/` (generated TLS certificates)

### 2. Interactive Setup (Recommended)

The **wizard** command walks you through the entire configuration process interactively:

**Server:**
```bash
smtptunnel-server wizard
```

The wizard will:
1. Ask for your server hostname (e.g., `mail.example.com`)
2. Ask for the listen address (default: `0.0.0.0:587`)
3. Generate TLS certificates automatically in `/etc/smtptunnel/certs/<hostname>/`
   - Or let you select existing certificate files
4. Create users with auto-generated secrets
5. Configure stealth/DPI evasion settings
6. Output a ready-to-use `config_<random_id>.toml` file

**Client:**
```bash
smtptunnel-client wizard
```

The wizard will:
1. Ask for the server address and credentials
2. Configure TLS certificate verification
3. Set up SOCKS5 proxy listeners
4. Set up port forwarding rules (optional)
5. Configure stealth settings
6. Output a ready-to-use `config_<random_id>.toml` file

### 3. Manual Setup

#### Server Setup (VPS)

1. **Generate Certificates:**
   ```bash
   smtptunnel-server gencerts -hostname mail.example.com
   ```
   Certificates are saved to `/etc/smtptunnel/certs/mail.example.com/` by default.
   Use `-output-dir <path>` to specify a custom location.

2. **Add a User:**
   ```bash
   smtptunnel-server adduser alice -c config.toml
   ```
   A secret is auto-generated and printed. Share this with the client user.

3. **Start Server:**
   ```bash
   smtptunnel-server run -c config.toml
   ```

#### Client Setup (Local)

1. Copy `ca.crt` from the server's cert directory to the client machine.
2. Edit `config.toml` with server address, username, and secret.
3. Configure SOCKS5 proxies and/or port forwarding rules.
4. **Connect:**
   ```bash
   smtptunnel-client run -c config.toml
   ```

---

## ⚙️ Configuration Reference (`config.toml`)

The project uses a unified TOML configuration file. Below is a complete reference with all available options.

### `[server]` — Server Settings

```toml
[server]
listen = "0.0.0.0:587"          # Interface and port to listen on
hostname = "mail.example.com"    # SMTP banner hostname (should match SSL cert)
cert_file = "/etc/smtptunnel/certs/mail.example.com/server.crt"
key_file = "/etc/smtptunnel/certs/mail.example.com/server.key"
log_level = "info"               # Logging level: info, debug, warn
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `listen` | Yes | `0.0.0.0:587` | Address and port to bind |
| `hostname` | Yes | `mail.example.com` | SMTP banner hostname |
| `cert_file` | Yes | — | Path to TLS server certificate |
| `key_file` | Yes | — | Path to TLS server private key |
| `log_level` | No | `info` | Logging verbosity |

### `[server.tls]` — TLS Settings

```toml
[server.tls]
min_version = "1.2"   # Minimum TLS version
```

### `[[server.users]]` — User Entries

Each user has a username, secret, optional IP whitelist, and logging flag.

```toml
[[server.users]]
username = "alice"
secret = "generated-base64url-secret"
whitelist = ["0.0.0.0/0"]    # Allow all IPs (default)
logging = true                # Enable traffic logging
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `username` | Yes | — | Unique username |
| `secret` | Yes | — | Shared secret (auto-generated with `adduser`) |
| `whitelist` | No | `["0.0.0.0/0"]` | Allowed client IP ranges (CIDR or single IP) |
| `logging` | No | `true` | Whether to log this user's connections |

You can add multiple `[[server.users]]` blocks. Users can also be managed via CLI:
```bash
smtptunnel-server adduser <name> -c config.toml
smtptunnel-server deluser <name> -c config.toml
smtptunnel-server listusers -c config.toml [-v]
```

### `[client]` — Client Settings

```toml
[client]
server = "mail.example.com:587"
username = "alice"
secret = "the-secret-from-adduser"
ca_cert = "ca.crt"
insecure_skip_verify = false
reconnect_delay = "2s"
max_reconnect_delay = "30s"
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `server` | Yes | — | Server address (`host:port`) |
| `username` | Yes | — | Authentication username |
| `secret` | Yes | — | Shared authentication secret |
| `ca_cert` | No | — | Path to CA certificate for TLS verification |
| `insecure_skip_verify` | No | `false` | Skip certificate validation (not recommended) |
| `reconnect_delay` | No | `2s` | Initial reconnection delay |
| `max_reconnect_delay` | No | `30s` | Maximum reconnection delay (exponential backoff) |

### `[[client.socks]]` — SOCKS5 Proxy Listeners

Define one or more local SOCKS5 proxy endpoints:

```toml
[[client.socks]]
listen = "127.0.0.1:1080"
username = ""     # Optional SOCKS5 authentication
password = ""
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `listen` | Yes | — | Local address:port to listen on |
| `username` | No | — | SOCKS5 auth username (empty = no auth) |
| `password` | No | — | SOCKS5 auth password |

**Usage with applications:**
```bash
# curl
curl -x socks5h://127.0.0.1:1080 https://ifconfig.me

# Firefox: Settings → Network → Manual Proxy → SOCKS Host: 127.0.0.1, Port: 1080

# SSH
ssh -o ProxyCommand='nc -x 127.0.0.1:1080 %h %p' user@remote
```

### `[[client.forward]]` — Port Forwarding Rules

Define one or more port forwarding rules for direct traffic redirection. Traffic received on the local `listen` address is forwarded through the tunnel to the remote `forward` destination.

```toml
[[client.forward]]
listen = "127.0.0.1:8080"       # Local listen address
forward = "internal-server:80"   # Remote destination through tunnel
protocol = "tcp"                 # "tcp" or "udp"
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `listen` | Yes | — | Local address:port to listen on |
| `forward` | Yes | — | Remote destination host:port (resolved on server side) |
| `protocol` | No | `tcp` | Protocol: `tcp` or `udp` |

**Examples:**

```toml
# Forward local port 3306 to a remote MySQL server
[[client.forward]]
listen = "127.0.0.1:3306"
forward = "db.internal:3306"
protocol = "tcp"

# Forward local port 8443 to a remote HTTPS server
[[client.forward]]
listen = "127.0.0.1:8443"
forward = "web.internal:443"
protocol = "tcp"

# Forward UDP traffic (e.g., DNS)
[[client.forward]]
listen = "127.0.0.1:5353"
forward = "8.8.8.8:53"
protocol = "udp"
```

**When to use forward vs SOCKS5:**
- Use **SOCKS5** when applications support SOCKS proxy (browsers, curl, etc.) and you want to access any destination dynamically.
- Use **forward** for applications that don't support SOCKS, or when you want a fixed local-to-remote port mapping (database connections, RDP, custom protocols, UDP traffic).

### `[stealth]` — DPI Evasion Settings

```toml
[stealth]
enabled = true
min_delay_ms = 50
max_delay_ms = 500
padding_sizes = [4096, 8192, 16384, 32768]
dummy_probability = 0.1
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `enabled` | No | `true` | Enable traffic shaping and padding |
| `min_delay_ms` | No | `50` | Minimum random delay between packet groups (ms) |
| `max_delay_ms` | No | `500` | Maximum random delay between packet groups (ms) |
| `padding_sizes` | No | `[4096, ...]` | Target packet sizes for padding |
| `dummy_probability` | No | `0.1` | Probability (0.0–1.0) of sending dummy packets during idle |

---

## 🔧 Complete Command Reference

### Server Commands

| Command | Description |
|---------|-------------|
| `run -c <config>` | Start the tunnel server |
| `adduser <name> -c <config>` | Add a new user with auto-generated secret |
| `deluser <name> -c <config>` | Remove a user |
| `listusers -c <config> [-v]` | List all users (add `-v` for details) |
| `gencerts -hostname <host>` | Generate CA + server TLS certificates |
| `check-config -c <config>` | Validate configuration file |
| `install` | Install binary to `/usr/local/bin/` and create directories |
| `wizard` | Interactive configuration generator |
| `service install <config>` | Register config as a systemd service |
| `service list` | List all registered smtptunnel services |
| `service remove <name>` | Stop, disable, and remove a service |
| `service logs <name> [-n N]` | View last N log lines (default: 50) |
| `service stop <name>` | Stop a running service |
| `service restart <name>` | Restart a service |
| `version` | Show version |

### Client Commands

| Command | Description |
|---------|-------------|
| `run -c <config>` | Connect and start SOCKS5/forward proxies |
| `ping -c <config> [-n N]` | Measure RTT latency (N pings, default: 4) |
| `status -c <config>` | Connection diagnostics |
| `check-config -c <config>` | Validate configuration file |
| `install` | Install binary to `/usr/local/bin/` and create directories |
| `wizard` | Interactive configuration generator |
| `service install <config>` | Register config as a systemd service |
| `service list` | List all registered smtptunnel services |
| `service remove <name>` | Stop, disable, and remove a service |
| `service logs <name> [-n N]` | View last N log lines (default: 50) |
| `service stop <name>` | Stop a running service |
| `service restart <name>` | Restart a service |
| `version` | Show version |

---

## 📦 Systemd Service Management

The `service` command group provides full lifecycle management of systemd services.

### Registering a Service

```bash
# After generating a config (via wizard or manually):
sudo smtptunnel-server service install config_abc123.toml
```

This will:
1. Copy the config file to `/etc/smtptunnel/configs/config_abc123.toml`
2. Create a systemd unit file at `/etc/systemd/system/smtptunnel-server-config_abc123.service`
3. Run `systemctl daemon-reload`
4. Enable and start the service immediately

### Managing Services

```bash
# List all registered services with their status
sudo smtptunnel-server service list

# View logs (last 100 lines)
sudo smtptunnel-server service logs smtptunnel-server-config_abc123 -n 100

# Stop a service
sudo smtptunnel-server service stop smtptunnel-server-config_abc123

# Restart a service
sudo smtptunnel-server service restart smtptunnel-server-config_abc123

# Completely remove a service (stops, disables, deletes unit file)
sudo smtptunnel-server service remove smtptunnel-server-config_abc123
```

### Running Multiple Instances

You can run multiple tunnel instances simultaneously by registering different configs:

```bash
# Create multiple configs via wizard
smtptunnel-server wizard   # -> config_aaa111.toml
smtptunnel-server wizard   # -> config_bbb222.toml

# Register each as a separate service
sudo smtptunnel-server service install config_aaa111.toml
sudo smtptunnel-server service install config_bbb222.toml

# List all running instances
sudo smtptunnel-server service list
```

---

## 🧙 Configuration Wizard

The wizard provides a guided, interactive setup experience. It is the recommended way to create configuration files.

### Server Wizard

```bash
smtptunnel-server wizard
```

**What it does:**
1. **Hostname** — Asks for the SMTP hostname (e.g., `mail.example.com`)
2. **Listen address** — Server bind address (default: `0.0.0.0:587`)
3. **Certificates** — Checks if certs exist at `/etc/smtptunnel/certs/<hostname>/`:
   - If yes: asks whether to reuse or regenerate
   - If no: offers to generate new CA + server certificates automatically
   - Or: lets you specify paths to existing certificate files
4. **Users** — Creates one or more users with auto-generated secrets
5. **Stealth** — Configures DPI evasion settings
6. **Output** — Writes `config_<random_id>.toml` in the current directory

### Client Wizard

```bash
smtptunnel-client wizard
```

**What it does:**
1. **Server address** — Remote server `host:port`
2. **Credentials** — Username and secret (from the server's `adduser` output)
3. **TLS** — Path to CA certificate or skip for insecure mode
4. **SOCKS5 proxies** — Add one or more local SOCKS5 listeners
5. **Port forwarding** — Add TCP/UDP forwarding rules
6. **Stealth** — DPI evasion settings
7. **Output** — Writes `config_<random_id>.toml` in the current directory

---

## 🔀 Port Forwarding (Forward Mode)

The **forward** feature allows transparent port forwarding through the tunnel, complementing the SOCKS5 proxy. This is useful for:

- Applications that don't support SOCKS proxies
- Fixed port mappings (databases, internal APIs, RDP)
- UDP traffic forwarding (DNS, game servers, etc.)

### How It Works

```
┌──────────┐     ┌─────────────────┐     ┌──────────────┐     ┌───────────────┐
│ App      │────▶│ Client Forward  │────▶│ SMTP Tunnel  │────▶│ Destination   │
│ :3306    │TCP  │ 127.0.0.1:3306  │SMTP │ Server       │TCP  │ db.internal   │
│          │◀────│                 │◀────│              │◀────│ :3306         │
└──────────┘     └─────────────────┘     └──────────────┘     └───────────────┘
```

1. Client listens on the local `listen` address
2. When a connection arrives, it opens a tunnel channel to `forward` destination
3. Data flows bidirectionally through the encrypted SMTP tunnel
4. On the server side, the connection is made to the final destination

### Configuration Example

```toml
# Access a remote database through the tunnel
[[client.forward]]
listen = "127.0.0.1:3306"
forward = "mysql.internal:3306"
protocol = "tcp"

# Access a remote web application
[[client.forward]]
listen = "127.0.0.1:8080"
forward = "webapp.internal:80"
protocol = "tcp"
```

Then connect your application to `127.0.0.1:3306` as if the database were local.

---

## 🛡️ Technical Overview

### SMTP Handshake

The tunnel establishes a legitimate-looking SMTP session:
1. Server sends `220 ESMTP` banner (mimics Postfix).
2. Client sends `EHLO`.
3. Server offers `STARTTLS`.
4. TLS handshake completed.
5. Client authenticates via a custom `AUTH PLAIN` token (HMAC-SHA256).
6. Communication switches to **binary mode** for data streaming.

### Binary Protocol

After the handshake, the tunnel uses an efficient binary framing protocol:

| Field | Size | Description |
|-------|------|-------------|
| Type | 1 byte | Frame type (DATA, CONNECT, CLOSE, PING, PONG) |
| Channel ID | 2 bytes | Multiplexed channel identifier |
| Payload Length | 2 bytes | Length of payload data |
| Payload | N bytes | Frame data |

**Supported frame types:**
- `DATA (0x01)` — Tunnel data
- `CONNECT (0x02)` — Open a new channel
- `CONNECT_OK (0x03)` — Channel opened successfully
- `CONNECT_FAIL (0x04)` — Channel open failed
- `CLOSE (0x05)` — Close a channel
- `PING (0x06)` / `PONG (0x07)` — Latency measurement

### Encryption

- **Transport layer:** TLS 1.2+ (via SMTP STARTTLS)
- **Application layer:** ChaCha20-Poly1305 (defense-in-depth)
- **Key derivation:** HKDF-SHA256 with per-direction keys
- **Authentication:** HMAC-SHA256 tokens with timestamp-based anti-replay

### Directory Structure

```
/usr/local/bin/
├── smtptunnel-server          # Server binary
└── smtptunnel-client          # Client binary

/etc/smtptunnel/
├── configs/                   # Per-instance configuration files
│   ├── config_abc123.toml     # Registered service configs
│   └── config_def456.toml
└── certs/                     # TLS certificates by hostname
    └── mail.example.com/
        ├── ca.key             # CA private key
        ├── ca.crt             # CA certificate (copy to clients)
        ├── server.key         # Server private key
        └── server.crt         # Server certificate
```

---

## 📋 Full Setup Walkthrough

This section provides a complete, step-by-step guide for setting up a tunnel from scratch.

### Step 1: Build and Install (Both Machines)

```bash
git clone https://github.com/youruser/smtptunnel.git
cd smtptunnel
make build
sudo ./smtptunnel-server install   # On server
sudo ./smtptunnel-client install   # On client
```

### Step 2: Server Configuration

**Option A — Wizard (recommended):**
```bash
sudo smtptunnel-server wizard
```
Follow the prompts. Note the generated username and secret.

**Option B — Manual:**
```bash
# Generate certificates
sudo smtptunnel-server gencerts -hostname mail.yourdomain.com

# Create config
cat > server-config.toml << 'EOF'
[server]
listen = "0.0.0.0:587"
hostname = "mail.yourdomain.com"
cert_file = "/etc/smtptunnel/certs/mail.yourdomain.com/server.crt"
key_file = "/etc/smtptunnel/certs/mail.yourdomain.com/server.key"
log_level = "info"

[server.tls]
min_version = "1.2"

[stealth]
enabled = true
min_delay_ms = 50
max_delay_ms = 500
padding_sizes = [4096, 8192, 16384, 32768]
dummy_probability = 0.1
EOF

# Add a user
sudo smtptunnel-server adduser alice -c server-config.toml
# Note the generated secret!
```

### Step 3: Register as Systemd Service (Server)

```bash
sudo smtptunnel-server service install server-config.toml
```

The server is now running and will auto-start on boot.

### Step 4: Client Configuration

Copy `/etc/smtptunnel/certs/mail.yourdomain.com/ca.crt` from the server to the client machine.

**Option A — Wizard (recommended):**
```bash
smtptunnel-client wizard
```
Enter the server address, username, and secret when prompted.

**Option B — Manual:**
```toml
# client-config.toml

[client]
server = "your-server-ip:587"
username = "alice"
secret = "the-secret-from-step-2"
ca_cert = "/path/to/ca.crt"
insecure_skip_verify = false
reconnect_delay = "2s"
max_reconnect_delay = "30s"

# SOCKS5 proxy for general browsing
[[client.socks]]
listen = "127.0.0.1:1080"

# Port forward for database access
[[client.forward]]
listen = "127.0.0.1:3306"
forward = "db.internal:3306"
protocol = "tcp"

[stealth]
enabled = true
min_delay_ms = 50
max_delay_ms = 500
padding_sizes = [4096, 8192, 16384, 32768]
dummy_probability = 0.1
```

### Step 5: Start the Client

```bash
# Direct run:
smtptunnel-client run -c client-config.toml

# Or register as service:
sudo smtptunnel-client service install client-config.toml
```

### Step 6: Verify

```bash
# Test SOCKS5 proxy
curl -x socks5h://127.0.0.1:1080 https://ifconfig.me

# Test port forwarding
mysql -h 127.0.0.1 -P 3306 -u dbuser -p

# Ping through tunnel
smtptunnel-client ping -c client-config.toml

# Check connection status
smtptunnel-client status -c client-config.toml
```

---

## 📄 License

Educational and authorized use only. Use responsibly.
