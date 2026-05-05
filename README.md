# dashBoard2go

**dashBoard2go** is a free, open-source web server control panel (cPanel & ISPConfig alternative) written natively in Go. Designed to be blazingly fast, highly concurrent, and flexible.

## Live Repository
Find the latest source and updates at: [https://github.com/sickplanet/dashBoard2go](https://github.com/sickplanet/dashBoard2go)

## Features & Implemented Capabilities

dashBoard2go is actively developed and currently supports the following enterprise-grade features:

- **Interactive Installer & Systemd Bootstrapper (`dashboard2go-setup`)**
  - Headless deployment style: interactively prompts for Hostname, Stack preference (Nginx vs Apache, Postgres), and Admin passwords.
  - Automatically fetches dependencies via `apt`, compiles binaries, wires up UFW firewall rules, and generates rigid systemd hooks.

- **Asynchronous Worker Queue (`dashboard2go-worker`)**
  - Tasks (like creating Linux user sandboxes, vhosts, or FTP accounts) are queued natively into a highly-concurrent SQLite WAL (Write-Ahead Logging) database.
  - The decoupled worker processes JSON payloads silently in the background, preventing your HTTP API from ever freezing during heavy disk I/O.

- **Automated Watchdog & Log Analyzer (`dashboard2go-watchdog`)**
  - Operates as an independent systemd daemon.
  - Recursively scans the `/home/dashboard2go/users/{user}/web/{domain}/logs/error.log` directories.
  - Pushes customizable visual alerts directly to the user's dashboard (Errors, Warnings, Both) based on their SQLite `user_alert_prefs`.

- **Strict Environment Sandboxing (ISPConfig/cPanel Style)**
  - Fully sandboxes system directories globally as `/home/dashboard2go/users/{username}/`.
  - Maps separated document roots (`public_html`), standard domains, log folders, and temp/session directories.
  - Automatically enforces strict PHP `open_basedir` constraints alongside decoupled FastCGI sockets (`pm = ondemand`) natively for PHP 8.3.

- **High-Performance Core API & Web Panel (`dashboard2go-core`)**
  - **SNI-Aware Reverse Proxy:** Bootstraps unencrypted HTTP (`:8080`) while seamlessly offering secure SNI HTTPS mapping on `:8443`.
  - **Modern UI Framework:** A built-in dual-themed (Light/Dark mode) Administration and Client Dashboard rendered via vanilla HTML/JS and completely responsive CSS variables.
  - **Live Monitoring:** The Admin Dashboard continually polls API endpoints returning real-time `systemctl is-active` statuses for Nginx, Apache2, MariaDB, PostgreSQL, Bind9, Postfix, etc.

## Requirements

- A fresh (brand-new) Debian (strongly recommended Debian 12/13) or Ubuntu VPS installation.
- `root` privileges.

## Installation

1. Log into your VPS as `root`.
2. Clone the repository and run the bootstrapper:

```bash
apt-get update -y && apt-get install -y git
git clone https://github.com/sickplanet/dashBoard2go.git
cd dashBoard2go
chmod +x install.sh
./install.sh
```

During installation, the Go compiler will automatically compile the four core components (`core`, `setup`, `watchdog`, `worker`), move the Web UI to standard paths, inject base templates, create the SQLite databases, and lock out further installations.

## Architecture

Please review `/docs/architecture.md` for a full breakdown of the Guard Service, SQLite embedded states, and API Routing layouts.

---
