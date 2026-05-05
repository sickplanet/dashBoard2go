## Plan: dashBoard2go Architecture & Roadmap

dashBoard2go is a free, open-source web server control panel (cPanel alternative) written in Go (JSON HTTP API) with an HTML/CSS/JS frontend. It natively targets Debian-based systems while maintaining an abstracted core architecture for future cross-platform (e.g., Windows) compatibility. It heavily draws inspiration from cPanel's feature set and flexibility.


**General Development Rules**
- **TODO Requirement**: Every time a mock function, UI event, or placeholder API route is added, a complete `// TODO: [Description of missing logic/backend requirements]` *must* be added next to it. This ensures all needed info is present for future implementation phases.

**Architecture Decisions**
- **Process Model (4-Tier Architecture)**:
  1. `dashboard2go-core`: The main HTTP API and UI web server. Fast, instantaneous response.
  2. `dashboard2go-worker`: Asynchronous job executor. Picks up tasks added to the SQL queue by the Core (e.g., "create vhost", "restart apache") and executes them via `internal/wrappers`. Mimics ISPConfig's deferred execution mode.
  3. `dashboard2go-watchdog`: Monitors service health (Nginx, MariaDB, FTP) and repairs configuration drift (e.g., UFW rule resyncs).
  4. `dashboard2go-updater`: Handles GitHub-based direct self-updating, migrations (SQL/JSON), and process restarts across versions.
- **Service Interfaces (Wrappers)**: Implement a strict "Strategy Pattern" using Go interfaces for ALL system services (Web Server, DB, Mail, DNS, Firewall). This ensures seamless interchangeability. 
- **Database Engine**: MariaDB/MySQL is **always** installed natively for users. PostgreSQL is provided optionally during setup if users want multi-DB support.
- **Embedded Control DB**: Use an embedded `SQLite` database with `PRAGMA journal_mode=WAL;` to cleanly allow all 4 Go processes to read/write concurrently without locking issues.
- **Setup Lock mechanism**: Post-installation, core hardcoded configs and an `installed: true` flag are saved in `config.json` next to the binaries. This prevents catastrophic duplicate setups.
- **Routing & Networking**: The Panel native interface binds to ports `8080` (HTTP) and `8443` (HTTPS).
- **Core FQDN & AutoSSL**: During setup, an FQDN virtualhost is generated. If Let's Encrypt is enabled on the FQDN:
  - We use the `webroot` validation plugin so we do *not* have to cycle web proxies dynamically (preventing downtime). 
  - FQDN SSL certs are pulled and subsequently reused globally across Mail Daemons (Postfix/Dovecot/Pure-FTPd).
  - The FQDN root provides a direct HTML status page outputting Watchdog interrupt history and login portals. 
  - Standard user domains inverse-proxy standard accesses (`userdomain.tld/dashboard2go`) straight to the `localhost:8080/8443` Go core.

**Steps**

**Phase 1: Foundation & Project Structure (✅ COMPLETED)**
1. ✅ Initialize Go modules and basic API framework.
2. ✅ Scaffold Domain-Driven Design layout (`/cmd/core`, `/cmd/watchdog`, `/internal/api`, `/internal/wrappers`).
3. ✅ Setup the separation of Admin vs. User routing logic and middleware (JWT + RBAC).
4. ✅ Create the frontend scaffold (HTML/JS/CSS), split into `/admin` and `/user`.

**Phase 2: The Bootstrap & Interactive Installation System (✅ COMPLETED)**
5. ✅ Create a CLI `setup` command that prompts for base configurations (hostname, IPv6 toggle, admin credentials) using `curl ifconfig.me` for dynamic IP resolution.
6. ✅ Implement `FIX BROKEN` installation bypass logic for developer environment recovery.
7. ✅ Dynamically generate authoritative Bind9 Micro-Zones (`db.fqdn`, `db.ns1`) locally *before* Let's Encrypt executes to solve Certbot `--standalone` DNS query timeouts.
8. ✅ Implement `internal/os/exec` wrappers for OS commands using proper `systemctl start` and `ufw allow 53` for DNS traversal.
9. ✅ Create `install.sh` bash script / `README.md` instructions enforcing `curl` and `ca-certificates` as aptitude prerequisites.

**Phase 3: Core Service Management Wrappers (cPanel Parity) (✅ COMPLETED)**
9. ✅ **Web Server Wrappers**: Implement `WebServer` interface for both `ApacheWrapper` and `NginxWrapper` (vhost generation, SSL linking).
10. ✅ **Database Wrappers**: Implement `Database` interface for `MariaDBWrapper` and `PostgresWrapper` (users, permissions, DBs).
11. ✅ **DNS Wrappers**: Implement DNS zone generation logic (Bind9/named) with DNSSEC & DKIM generation support.

**Phase 4: Email & Webmail Integration (✅ COMPLETED)**
12. ✅ **Mail Stack Wrappers**: Implement `MailServer` wrappers (Postfix, Dovecot, Amavisd, SpamAssassin) to manage virtual domains, mailboxes, aliases, and spam filter toggles.
13. ✅ **Webmail Routing**: Implement `webmail.domain.tld` alias logic across Apache/Nginx. Unmapped routes display an app chooser (Roundcube vs. others), mapped paths route directly to the specific webmail client.
14. ✅ **Roundcube Manager**: Build wrapper utility for Admins to seamlessly install/upgrade Roundcube's core, themes, and plugins from remote sources.

**Phase 5: Watchdog & Guard Service (✅ COMPLETED)**
15. ✅ Build the standalone `guard-service` process to poll daemons (`cmd/watchdog/main.go`).
16. ✅ Implement polling and parsing of process statuses (`systemctl is-active`) via OS wrappers.
17. ✅ Implement the Rules Engine: Restart service if down, log failures.
18. ✅ Build Admin UI mapping connecting API to Web Server to see live polling output dynamically.

**Phase 6: Accounts, User Interface, & Theming (✅ COMPLETED)**
17. ✅ Build API endpoints for full account lifecycle (Create, Suspend, Terminate) — allocating OS users, quotas, and web home directories.
18. ✅ Build the Web UI dashboards mapping closely to expected cPanel features (File Manager, phpMyAdmin auto-login, Email Accounts).
19. ✅ Implement Dynamic Theming Engine: Integrate Bootstrap 5 layout system across Admin & User portals utilizing `data-bs-theme="dark"` defaults.
20. ✅ Build Theme Settings Portal: Create an Admin interface (`themes.html`) to configure default system themes, allow layout overrides per user category (Resellers vs Clients), and inject dynamic `.js` theming across all pages. 

**Phase 7: Security & Firewall**
21. Define the `Firewall` interface and implement via `UFW`/`iptables`. Must ensure rule persistence by tracking rule signatures in the SQL database. If UFW state deviates from the SQL configuration (i.e. rules change their IDs or get lost), the Watchdog will issue an alert for the Admin and provide an auto-reconfiguration mechanism.
22. SSL/TLS Certificate Manager: Build domain/subdomain interfaces granting granular controls to disable SSL, use Custom Certificates, or automatically provision via Let's Encrypt (Certbot). Integrate automatic SSL setup with Apache/Nginx vhost wrappers.

**Phase 8: Job Queue & Worker Architecture**
23. Create `dashboard2go-worker` binary.
24. Implement Job Queue SQL model (`internal/queue`): The Core API submits actions (e.g., `ACTION_ADD_FTP_USER`, payload JSON).
25. Worker picks up jobs, locks them, maps the JSON payload to the `Wrappers`, executes, and records Success/Failure timestamps, leaving `dashboard2go-core` purely as a frontend API.

**Phase 9: Versioning, config.json, & Updater**
26. Build `internal/config/config.go` parser for a local `config.json` that sets `installed: true` to prevent destructive reruns.
27. Build `dashboard2go-updater` binary. Establish GitHub Release version checking. When an update is detected, the Updater spins up, downloads the new payload, shuts down the Core/Worker/Watchdog, runs SQL/JSON migrations, swaps binaries, and restarts them.

**Phase 10: Security & Final Audit**
1. **Unit tests**: Ensure configuration template generation (Nginx/Apache vhosts, Bind9 zones) matches expected format strings.
2. **Integration tests**: Use a Docker container to simulate a Debian environment and verify stack choices correctly install and start the chosen services (e.g., Nginx + MariaDB vs Apache + Postgres).


**Phase 11: Real-Time UI Database Bindings (✅ COMPLETED)**
28. ✅ Expand the Admin UI (`web/admin/index.html`) layout to support dynamic tracking of MySQL/PostgreSQL databases and Email mailboxes.
29. ✅ Create pure SQLite Native schema creation and `/api/v1/...` JSON GET/POST endpoints inside `internal/api/routes.go` for Domains, Databases, and Emails.
30. ✅ Refactor User UI (`web/user/index.html`) matching the Admin sleek sidebar aesthetic.
31. ✅ Wire Frontend Javascript `fetch()` POST logic replacing mock alerts to interact directly with the backend, appending to the `dashboard2go-worker` queue.
32. ✅ Create Bootstrap 5 Modal HTML forms for User-level Database and Email creation mirroring the Domain structure.

**Decisions**
- The project will use an abstracted `OSExecutor` interface to mock commands during tests.
- Support for Roundcube and SquirrelMail as built-in webmail clients to closely mimic cPanel.
- Standalone Watchdog process to ensure panel and hosted services robustness.