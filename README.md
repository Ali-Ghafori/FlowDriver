# FlowDriver 🌊

> **Optimized for AI CLI tools** — FlowDriver v1.3.2 has been specifically tuned for tunneling AI assistant traffic (Claude Code, OpenCode, Cursor, and others) through restrictive network environments. It achieves stable multi-hour sessions with minimal connection resets, making AI coding tools fully usable in censored regions.

**[فارسی](README.fa.md)** | English

---

**FlowDriver** is a covert TCP tunnel that routes network traffic through Google Drive. Instead of direct TCP connections, data is packed into binary files, uploaded to a shared Google Drive folder, and reassembled at the other end — making all traffic appear as normal Google Drive API usage.

## How It Works

```
[Your App] → SOCKS5 → [FlowDriver Client] → Google Drive ← [FlowDriver Server] → [Target Server]
```

1. **Client** listens on a local SOCKS5 port. When your app connects, it bundles the TCP stream into binary `.bin` files and uploads them to a shared Google Drive folder.
2. **Server** polls the same Drive folder. When it finds a request file, it downloads it, opens a real TCP connection to the target, and uploads the response back as another file.
3. Both sides delete files after reading them, keeping the Drive folder clean.

The connection looks like ordinary Google Drive API activity to any network monitor or DPI system.

## What's New in v1.3.2

- **AI CLI fix**: `VirtualConn.Write()` now returns `net.ErrClosed` on a closed session. Previously, HTTP/2 clients (Claude Code, OpenCode) would silently write to a dead session and get an immediate EOF on the next read, causing "socket connection closed unexpectedly" errors.
- **Zombie session fix**: Server-side `handleServerConn` now calls `CloseSession` instead of `RemoveSession` on exit. This sends a `Close=true` envelope to the client so HTTP/2 connections get a clean EOF rather than silently dying.
- **TOCTOU race fix**: Tombstone check and session map lookup are now atomic under `sessionMu`, preventing phantom session creation after cleanup.
- **Upload retry**: Failed uploads are retried up to 3 times with exponential backoff before data is dropped, with a warning log.
- **404 handling**: Download 404 errors (cleanup race) are distinguished from transient errors and no longer trigger an infinite retry loop.
- **Log file support**: New `-log <path>` flag writes logs to a file while also printing to stdout.
- **Token refresh logging**: OAuth2 token refresh events are now logged with timestamps.
- **Goroutine leak fix**: Pipe-writer goroutines in `Upload` are now properly unblocked on HTTP error.

## Quick Start

Download the latest pre-built binaries from the [Releases](../../releases) page.

### 1. Get Google Drive Credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/), create a project, and enable the **Google Drive API**.
2. Go to **APIs & Services → OAuth consent screen**. Fill in app name and support email, then click **Publish App** (prevents token expiry every 7 days).
3. Go to **Credentials → Create Credentials → OAuth client ID**. Select **Desktop App**.
4. Download the JSON file and save it as `credentials.json`.

### 2. Configure

**Client** (`client_config.json`):
```json
{
  "listen_addr": "127.0.0.1:1080",
  "storage_type": "google",
  "refresh_rate_ms": 400,
  "flush_rate_ms": 800,
  "idle_timeout_sec": 900,
  "transport": {
    "TargetIP": "216.239.38.120:443",
    "SNI": "google.com",
    "HostHeader": "www.googleapis.com",
    "InsecureSkipVerify": false
  }
}
```

The `transport` block enables **domain fronting** — all HTTPS traffic routes through Google's IPs, bypassing IP-based blocking. Remove this block if you are not behind censorship.

Leave `google_folder_id` empty — the tool will automatically find or create a folder named **Flow-Data** and save its ID to your config on first run.

**Server** (`server_config.json`):
```json
{
  "storage_type": "google",
  "refresh_rate_ms": 400,
  "flush_rate_ms": 800,
  "idle_timeout_sec": 900
}
```

### 3. First-Time Authentication (Client Machine)

Run the client once interactively to authorize Google Drive access:

```bash
./client -c client_config.json -gc credentials.json
```

A URL will appear in the terminal. Open it in a browser, log in to your Google account, grant permissions, then copy the full redirect URL from your browser's address bar and paste it back into the terminal.

A `.token` file is created next to `credentials.json`. This file contains your refresh token — copy both `credentials.json` and the `.token` file to the server.

### 4. Run

**Server** (on your upstream machine):
```bash
./server -c server_config.json -gc credentials.json -log server.log
```

**Client** (on your local machine):
```bash
./client -c client_config.json -gc credentials.json -log client.log
```

The client listens on `127.0.0.1:1080` (SOCKS5). Point your application at that address.

## Using with AI CLI Tools

### Claude Code
```bash
export HTTPS_PROXY=socks5://127.0.0.1:1080
claude
```

### OpenCode
Set SOCKS5 proxy in your OpenCode config or environment:
```bash
export HTTPS_PROXY=socks5://127.0.0.1:1080
opencode
```

### With GOST (HTTP→SOCKS5 bridge)
If your tool only supports HTTP proxy, use [GOST](https://github.com/go-gost/gost) as a bridge:
```bash
gost -L http://:8080 -F socks5://127.0.0.1:1080
```
Then set `HTTPS_PROXY=http://127.0.0.1:8080`.

## Performance & Quotas

Google Drive has strict API rate limits. Recommended configuration for AI API traffic:

| Setting | Recommended | Notes |
|---|---|---|
| `refresh_rate_ms` | 400 | Poll interval for incoming data |
| `flush_rate_ms` | 800 | How often TX buffer is uploaded |
| `idle_timeout_sec` | 900 | 15 minutes — long enough for streaming AI responses |

- Do not set `refresh_rate_ms` below 100ms — you will exhaust your daily quota.
- The server auto-backs off polling to 5s when there are no active sessions, reducing API usage to near zero at idle.

## Build from Source

```bash
git clone https://github.com/Ali-Ghafori/FlowDriver
cd FlowDriver
go build -ldflags "-X main.version=v1.3.2" -o client ./cmd/client
go build -ldflags "-X main.version=v1.3.2" -o server ./cmd/server
```

Requires Go 1.22 or later.

## Flags

```
-c    Path to config file         (default: config.json)
-gc   Path to credentials.json    (default: credentials.json)
-log  Path to log file            (default: stdout only)
```

---

## Disclaimer

This project is for personal and research use only. Do not use it for illegal purposes or in production environments. The authors accept no responsibility for misuse.
