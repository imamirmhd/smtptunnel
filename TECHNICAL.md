# 📧 SMTP Tunnel - Technical Documentation (Go Edition)

This document provides in-depth technical details about the SMTP Tunnel Proxy, including protocol design, DPI evasion techniques, security analysis, and implementation details.

> 📖 For basic setup and usage, see [README.md](README.md).

---

## 📑 Table of Contents

- [📨 Why SMTP?](#-why-smtp)
- [🎭 How It Bypasses DPI](#-how-it-bypasses-dpi)
- [🏗️ Architecture (Go)](#️-architecture-go)
- [📐 Protocol Design](#-protocol-design)
- [🔧 Internal Component Details](#-internal-component-details)
- [🔐 Security Analysis](#-security-analysis)
- [⚙️ Automation & Orchestration](#️-automation--orchestration)
- [📡 Network Performance](#-network-performance)

---

## 📨 Why SMTP?

SMTP (Simple Mail Transfer Protocol) is the protocol used for sending emails. It's an excellent choice for tunneling because:

### 1️⃣ Ubiquitous Traffic
- Email is essential infrastructure - blocking it breaks legitimate services.
- SMTP traffic on port 587 (submission) is expected and normal.

### 2️⃣ Expected to be Encrypted
- STARTTLS is standard for SMTP - encrypted email is normal.
- DPI systems expect to see TLS-encrypted SMTP traffic.

### 3️⃣ Flexible Protocol
- SMTP allows large data transfers (attachments).
- Binary data is normal (MIME-encoded attachments or binary extensions).

### 4️⃣ Hard to Block
- Blocking port 587 would break email for everyone.
- Can't easily distinguish tunnel from real email after TLS encryption.

---

## 🎭 How It Bypasses DPI

Deep Packet Inspection (DPI) systems analyze network traffic to identify and block certain protocols or content. SMTP Tunnel evades detection via a multi-phase handshake:

### 🔍 Phase 1: The Deception (Plaintext)

The server mimics a standard Postfix SMTP server:
```
Server: 220 mail.example.com ESMTP Postfix (Ubuntu)
Client: EHLO client.local
Server: 250-mail.example.com
        250-STARTTLS
        250-AUTH PLAIN LOGIN
        250 8BITMIME
Client: STARTTLS
Server: 220 2.0.0 Ready to start TLS
```

### 🔒 Phase 2: TLS Handshake

Standard TLS 1.2+ handshake occurs immediately after the `STARTTLS` command. DPI sees normal TLS negotiation for email encryption.

### 🚀 Phase 3: Binary Protocol Upgrade

Once the TLS tunnel is established, the client authenticates and upgrades to a binary framing protocol:
```
Client: EHLO client.local
Server: 250-mail.example.com
        250-AUTH PLAIN LOGIN
        250 8BITMIME
Client: AUTH PLAIN <hmac_sha256_token>
Server: 235 2.7.0 Authentication successful
Client: BINARY
Server: 299 Binary mode activated
```

**[Binary streaming begins - raw multi-channel tunnel]**

---

## 🏗️ Architecture (Go)

The project is implemented in Go, leveraging goroutines for high-concurrency multiplexing.

### 📡 Data Flow

```
YOUR COMPUTER                           YOUR VPS                        INTERNET
┌────────────────────┐                  ┌────────────────────┐          ┌─────────┐
│       App          │                  │                    │          │         │
│ (Browser/Netcat)   │                  │  ┌──────────────┐  │          │ Website │
└──────┬──────┬──────┘                  │  │    Server    │  │          │   API   │
       │      │                         │  │   Binary     │  │          │ Service │
SOCKS5 │      │ FWD                     │  └──────┬───────┘  │          └────┬────┘
       ▼      ▼                         │         │          │               │
┌──────────────┐      TLS Tunnel        │         │ TCP/UDP  │               │
│    Client    │◀───────────────────────┼────────▶│ Outbound │◀──────────────┘
│ (smtptunnel) │      Port 587          │         │ Connector│
└──────────────┘                        │  └──────────────┘  │
                                        └────────────────────┘
```

1. **Client** listens for local traffic (SOCKS5 or Port Forwarding).
2. **Client** establishes a single SMTP-cloaked TLS connection to the **Server**.
3. **Data** is wrapped in binary frames and multiplexed over the tunnel.
4. **Server** decapsulates frames and establishes outbound TCP/UDP connections to the final destination.

---

## 📐 Protocol Design

### 📦 Frame Format

All communication after the `BINARY` upgrade uses this fixed-header binary frame format:

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 1 byte | Type | Frame type identifier |
| 1 | 2 bytes | Channel ID | Multiplexed stream identifier (Big-endian) |
| 3 | 2 bytes | Length | Payload size (Big-endian) |
| 5 | N bytes | Payload | Data |

**Frame Types:**
- `0x01`: **DATA** — Tunnel data transmission.
- `0x02`: **CONNECT** — Request to open a new channel (host:port).
- `0x03`: **CONNECT_OK** — Channel successfully established.
- `0x04`: **CONNECT_FAIL** — Channel establishment failed (contains error message).
- `0x05`: **CLOSE** — Signal to close a channel.
- `0x06`: **PING** — Measure round-trip time.
- `0x07`: **PONG** — Respond to ping.

---

## 🔧 Internal Component Details

The project is structured into modular internal packages:

| Package | Responsibility |
|---------|----------------|
| `internal/config` | TOML structure definitions and validation. |
| `internal/proto` | Implementation of the binary framing protocol (Reader/Writer). |
| `internal/tunnel` | High-level session management (multiplexing, channel mapping). |
| `internal/smtp` | SMTP handshake implementation (mimics real mail servers). |
| `internal/socks5` | SOCKS5 proxy server implementation. |
| `internal/forward` | TCP and UDP local port forwarding listeners. |
| `internal/crypto` | Key derivation (HKDF-SHA256) and encryption (ChaCha20-Poly1305). |
| `internal/certs` | Self-signed CA and server certificate generation. |
| `internal/service` | Automation tools: Wizard, Binary Install, Systemd Service CRUD. |
| `internal/debug` | Diagnostics (Ping, Status checks, Config validation). |

---

## 🔐 Security Analysis

### 🛡️ Encryption Layers

1.  **TLS 1.2+ layer**: The primary encryption used for DPI evasion.
2.  **ChaCha20-Poly1305 layer**: Optional defense-in-depth encryption on top of TLS.
    - **Keys**: Derived from the shared secret using **HKDF-SHA256**.
    - **Nonce**: Constructed using a monotonically increasing sequence number combined with random bytes to prevent reuse.

### 🔑 Authentication

Authentication uses an `AUTH PLAIN` token exchange:
1.  **Token**: `base64(username + ":" + timestamp + ":" + hmac(secret, "smtp-tunnel-auth:" + username + ":" + timestamp))`
2.  **Anti-Replay**: The server validates the timestamp within a strict drifting window (±5 minutes).

### 🔍 Threat Model

| Threat | Mitigation |
|--------|------------|
| Passive eavesdropping | Mandatory TLS encryption for all traffic. |
| Active MITM | CA certificate pinning; mandatory verification against pinned `ca.crt`. |
| Replay attacks | HMAC-signed timestamps with short validity windows. |
| Active Probing | SMTP banner mimicry andCapability negotiation identical to Postfix. |

---

## ⚙️ Automation & Orchestration

SMTP Tunnel includes built-in orchestration tools to simplify deployment:

### 🧙 Configuration Wizard
The `wizard` subcommand provides an interactive setup for both server and client:
- **Server**: Generates hostname-based certificates, creates users, and formats TOML.
- **Client**: Connects to server, configures TLS, and sets up proxies/forwards.

### 📦 Systemd Management
The `service` command group manages the full lifecycle of tunnel instances:
- **`install`**: Registers a config file as a persistent system service.
- **`list`**: Shows status of all registered tunnel instances.
- **`logs/stop/restart`**: Wrappers for `journalctl` and `systemctl` tailored for the tunnel.

### 📂 Directory Structure
Standardized paths for Linux installations:
- `/usr/local/bin/`: Binaries.
- `/etc/smtptunnel/`: Root configuration.
- `/etc/smtptunnel/configs/`: Registered instance configurations.
- `/etc/smtptunnel/certs/`: Domain-isolated TLS certificate storage.

---

## 📡 Network Performance

### 🏗️ Concurrency Model
The Go implementation uses **Multiplexed Asynchronous I/O**. Each tunneled connection (channel) is managed by lightweight goroutines, allowing thousands of simultaneous streams over a single SMTP session with minimal memory overhead.

### 📈 Efficiency Improvements over v1.x
- **No Base64**: Traffic is sent as raw binary after the handshake.
- **Zero Round-Trips**: Data streaming is asynchronous and non-blocking.
- **Low Header Overhead**: 5-byte fixed header vs hundreds of bytes in standard SMTP.

---

## 📋 Version Information

- **Current Version:** 2.1.0
- **Implementation:** Go 1.18+
- **Protocol Version:** Binary streaming v2
- **Encryption:** TLS 1.2+ / ChaCha20-Poly1305
