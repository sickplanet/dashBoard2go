# dashBoard2go

**dashBoard2go** is a free, open-source web server control panel (cPanel & ISPConfig alternative) written natively in Go. Designed to be blazingly fast, highly concurrent, and flexible.

## Features

- **Monolithic Core with Watchdog**: A central API and task manager with a decoupled watchdog to ensure service resilience.
- **Go Interface Wrappers**: True "Strategy Pattern" wrapper design allowing you to hot-swap stack choices without core code logic changes.
- **Flexible Stack Support**: Choose `Nginx` or `Apache` for web handling, and `MariaDB` or `PostgreSQL` for databases mapping at install. 
- **cPanel/ISPConfig Inspired UI**: Familiar paradigms for users without the enterprise licensing fees.
- **Debian / Ubuntu First**: Tightly integrated with `apt` and `systemctl`.

## Requirements

- A fresh (brand-new) Debian or Ubuntu VPS installation.
- `root` privileges.

## Installation

1. Log into your VPS as `root`.
2. Run the frictionless installer script:

```bash
wget -O install.sh https://raw.githubusercontent.com/yourrepo/dashBoard2go/main/install.sh
chmod +x install.sh
./install.sh
```

During installation, you will be interactively prompted to choose your desired software stack (Web Server, Database, Mail System, Hostname).

## Architecture

Please review `/docs/architecture.md` for a full breakdown of the Guard Service, SQLite embedded states, and API Routing layouts.

---
*Status: Initial scaffold & Phase 2 Install System constructed.*