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
7. ✅ Dynamically generate authoritative Bind9 Micro-Zones (`db.fqdn`, `db.ns1`, `db.baseDomain`) locally parsing Primary and Secondary server IPs *before* Let's Encrypt executes to solve Certbot `--standalone` DNS query timeouts for delegated domains.
8. ✅ Implement `internal/os/exec` wrappers for OS commands using proper `systemctl start` and `ufw allow 53` for DNS traversal.
9. ✅ Create `install.sh` bash script / `README.md` instructions enforcing `curl` and `ca-certificates` as aptitude prerequisites.

**Phase 3: Core Service Management Wrappers & Package Domains (✅ COMPLETED)**
9. ✅ **Web Server Sub-Package**: Implement `webserver` isolating `ApacheWrapper`, `NginxWrapper` and `phpfpm.go`.
10. ✅ **Database Sub-Package**: Implement `database` isolating `MariaDBWrapper` and `PostgresWrapper`.
11. ✅ **DNS Sub-Package**: Implement `dns` isolating zone generation logic (`bind9.go`).
11.5. ✅ **SSL/Certbot Package**: Implement `ssl` mapping `SSLManager` (`certbot.go`, `acmesh.go`) to control Let's Encrypt webroot and standalone validations dynamically.
11.6. ✅ **Mail & FTP Packages**: Extracted standard Postfix/Dovecot logic into `mail`, and PureFTPd logic into `ftp`.

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
## Step 9: Database vs Engine UI Drift Synchronization
- [x] Create SQLite 0002 Embedded Migration for firewall_rules.
- [x] Sync ufw.go `GetDBRules` to read valid struct elements from 0002 dataset.
- [x] Display warnings inside `web/admin/index.html` for Missing rules (SQL drift).
- [x] Display warnings inside `web/admin/index.html` for Rogue rules (OS drift).
- [ ] TODO: connect Sync mechanics natively to standard backend wrapper routine.
- [ ] TODO: Implement API endpoints for "Save rules to SQL" and "Delete OS rules".

## Step 10: Setup Stability & Locale
- [x] Implement UI JS bindings `fetchServices()` inside `web/admin/index.html` to populate Watchdog monitoring card.
- [x] Configure system `en_US.UTF-8` locale proactively during `cmd/setup/main.go` prior to `apt-get` to fix perl warnings.
- [x] Fix Let's Encrypt `Port 80/TCP bind` error by stopping `nginx`/`apache2` before executing `certbot` standalone.
- [x] Add Watchdog `/services` endpoint fetching systemd states inside `internal/api/routes.go`.
- [x] Construct Watchdog services summary Dashboard card inside `web/admin/index.html`.

## Step 11: Route Stability & Endpoint Fixes
- [x] Fix `gin` router panics caused by identical route redeclarations (`/services`, `/firewall`).
- [x] Validate `/api/v1/admin/users`, `/api/v1/admin/updates`, and `/api/v1/admin/firewall` return valid 200 JSON payloads resolving the 404 admin interface tracking bugs.

## Step 11: Route Stability & Endpoint Fixes
- [x] Fix `gin` router panics caused by identical route redeclarations (`/services`, `/firewall`).
- [x] Validate `/api/v1/admin/users`, `/api/v1/admin/updates`, and `/api/v1/admin/firewall` return valid 200 JSON payloads resolving the 404 admin interface tracking bugs.

## Step 12: Ensure Rules Compliance
- [x] Add strict `// TODO:` inline comments to all placeholder API routes (`/api/v1/admin/status`, `/api/v1/user/alerts`) as per architecture rules.
- [x] Add missing `// TODO:` for the unimplemented Session Auth Middleware over the API groups.

## Step 13: DNS Tab Implementation
- [x] Implemented ISPConfig 2-column style UI matrix mapping inside `#dns` component (`web/admin/index.html`).
- [x] Embedded DNS Wizard Template schemas dynamically bound towards JSON payloads matching Database mapping formats per screenshots.
- [x] Included DNS scaffolding for the User Client GUI mapping explicitly towards Backend Bind9 / Zones.
- [x] Created `internal/api/routes.go` Admin + User REST controllers enforcing `// TODO` mappings strictly against `dns_templates` requirements.

## Step 14: System Configuration & APT Wrappers
- [x] Defined `internal/oswrap/apt.go` containing isolated `DEBIAN_FRONTEND=noninteractive` wrappers (`AptInstall`, `AptRemove`, `AptUpdate`, `AptUpgrade`) tied closely to `context.Context` to ensure safe operation.
- [x] Updated `rules.md` formally isolating OS operations away from HTTP REST constraints directly toward the asynchronous worker queue.
- [x] Established `/api/v1/admin/system/config` and `/api/v1/admin/system/apt` endpoints enforcing structural strictness using `// TODO` mappings.
- [x] Constructed `System Context` primary admin UI mirroring the multi-tab layout found in ISPConfig (Mail, Web, Server, DNS configuration schema matrices).
- [x] Bound internal forms logically against native JavaScript dispatchers (`saveServerConfig`, `execAPT`) properly simulating backend behavior pending worker integrations.

## Step 15: Self-Updating Architecture
- [x] Defined Rule #6 in `rules.md` explicitly defining decoupled self-updating bash paradigms preventing `text file busy` blocks during active daemon overwrites.
- [x] Implemented `internal/updater/apply.go` generating decoupled `/tmp/dashBoard2go-apply-update.sh` that safely unlinks and stops its own executing tree.
- [x] Updated `cmd/updater/main.go` to aggressively poll the SQLite database for an `apply_update` signal (triggered by UI).
- [x] Bound the `Update Available` green Admin UI menu button to `POST /api/v1/admin/system/update/apply` instructing the `dashboard2go-updater` daemon.

## Step 16: Router-Style Live Firmware Upgrade Interface
- [x] Fixed `fmt.Sprintf` escaping error where `%H:%M:%S` was interpreted as formatting verbs inside Go instead of standard shell Date syntax.
- [x] Redeployed `internal/updater/apply.go` Bash wrapper aggressively writing extraction steps locally utilizing `tee` against `/var/www/html/dashboard2go_update.log` mimicking standard ISP root exposure logic.
- [x] Established `GET /api/v1/admin/updates/log` extracting `/tmp/dashboard2go-update.log`.
- [x] Injected UI Javascript logic tracking decoupled firmware payloads. When dispatched, `web/admin/index.html` natively overlays the screen locking interactivity, concurrently downloading `http://$HOSTNAME/dashboard2go_update.log` through Nginx while pinging the backend API awaiting successful daemon restart confirmation parameters before rendering the final `Update Complete` exit mechanism.

## Step 17: Front-End UI Reliability & Sub-Package Re-Architecture
- [x] Restructured monolithic `ssl.go`, `bind9.go` code into highly-specialized domain subdirectories (`internal/wrappers/ssl`, `internal/wrappers/dns`, etc).
- [x] Resolved major `web/admin/index.html` runtime JavaScript panic that previously blocked all SPA interactions and polling logic natively inside the browser context due to merge artifacts triggering `Unexpected token ')'`.
- [x] Visually connected the detached Firmware self-update API mechanism explicitly to the front-end Administrator Interface through a dedicated `Panel Update` GUI settings pane underneath System Configuration options.

## Step 18: Front-End UI Javascript Re-Architecture
- [x] Extracted monolithic inline JavaScript from `web/admin/index.html` and `web/user/index.html`.
- [x] Restructured into `web/js/core.js` mapping global variables (`logout()`, `returnAdmin()`).
- [x] Defined `web/js/admin.js` for administrator restricted SPA interactions.
- [x] Defined `web/js/user.js` for standard user domains, database, and email payload processing.
- [x] Updated Gin-Gonic routes in `cmd/core/main.go` to serve `/js/*` static file paths publicly.

## Step 19: REST API Refactor
- [x] Analyzed `internal/api/routes.go` and rewrote the routing handlers to adhere securely strictly against standard `REST` HTTP Method architectures (e.g. replacing hardcoded `user.POST("/vhost")` directly to mapped `user.POST("/domains")`).
- [x] Implemented logic mapping inside `internal/api/routes.go` exposing `admin.POST("/updates/apply")`, `admin.PUT("/firewall/toggle")`, `admin.POST("/system/apt")` validating previously stubbed endpoints dynamically against existing Javascript mappings directly triggering background worker states. 
- [x] Rewrote all inline SPA Javascript bindings (`web/js/admin.js`, `web/js/user.js`, `web/login.html`) replacing bare `fetch` blocks with the new generic `apiFetch` routine (located in `web/js/core.js`) ensuring uniform JSON payload and request parameter mappings according strictly to the configured `REST` backend expectations.
- [x] Resolved edge case where backend failures generated HTML unmarshaling UI errors natively by patching `web/js/core.js` payload unwrapping mechanism so UI gracefully projects `.error` string. 

## Step 20: Governance & Architecture Standards
- [x] Defined Rule #7 rigidly enforcing standard RESTful API conventions (`GET`/`POST`/`PUT`/`DELETE`) globally across both Golang API handlers and explicit `apiFetch` dispatch configurations within `rules.md`.

## Step 21: BIND9 ACME DNS Resolution Fixes
- [x] Resolved Let's Encrypt CAA query timeouts inside `cmd/setup/main.go`.
- [x] Prevented `createMicroZone` from creating rogue, detached top-level zones for hostnames (`usaeast.zenhub.ro`) and nameservers (`ns1.zenhub.ro`, `ns2.zenhub.ro`).
- [x] Re-architected `setupLocalBindZone` to configure a single authoritative zone for the explicit `baseDomain` (`zenhub.ro`), injecting the FQDN and Nameservers as correct `A` sub-records underneath its umbrella.

## Step 22: DNS Propagation & Let's Encrypt Timeout Mitigation
- [x] Added `time.Sleep(5 * time.Second)` buffer after issuing `systemctl restart bind9` in `cmd/setup/main.go` to explicitly grant the BIND9 daemon sufficient network annunciation time to unpack physical zone files into memory over UDP/53 before Let's Encrypt immediately initiates multi-perspective HTTP-01 DNS evaluations.
- [x] Injected an explicit `NS` DNS record mapping `ns2` into the generated BIND9 Authoritative base zone avoiding `Lame Delegation` rejection logic native to Unbound validation routines.
- [x] Added prompt for IPv6 and automatic injection of AAAA records for FQDN, Nameservers and Base Domain
- [x] Fixed duplicate zone insertion bug in BIND wrapper: Added check to prevent writing a zone block in `named.conf.dashboard` if it already exists. Zone DB files are implicitly overwritten, fulfilling the "replace everything inside" flow without BIND process crashing.
- [x] Verified that returning `nil` in Go satisfies error interfaces as a "success" state, preventing duplicate zone crash errors in BIND config files.
- [x] Fixed Let's Encrypt validation DNS timeout: added missing BIND9 `systemctl restart bind9` & `time.Sleep` step directly before calling certbot. Let's Encrypt was validating before BIND explicitly loaded the zone.
- [x] Fixed an async daemon boot race condition where `cmd/setup/systemd.go` executed `systemctl start` before `config.json` received its fresh cryptographic pointers. `dashboard2go-core` was trapped running the HTTP-only routine from previous states.
- [x] Modified `systemd.go` to explicitly loop `systemctl restart` to ensure newly minted SSL directives enforce the listener on `8443`.
- [x] Modified `cmd/core/main.go` to explicitly parse direct absolute paths from `/etc/letsencrypt/live/<fqdn>/*.pem`, safely bypassing Go's strict internal TLS constraints.

## Step 23: Dashboard Updater Fix
- [x] Fixed 404 Network Error for the updater UI component by properly mapping the `admin.POST("/updates/apply")` REST endpoint in `internal/api/routes.go` which links frontend button clicks seamlessly to the decoupled `updater.ApplyUpdate()` background worker routine.
## Step 24: Updater Check Re-architecture
- [x] Added `admin.POST("/updates/check")` REST route natively forcing `updater.CheckForUpdates()` which requests latest generic JSON info securely from the GitHub API and parses metadata downstream to the frontend.
- [x] Converted the single blind static install button into a decoupled verification step: users hit `Check for Updates`, JS reads the new endpoint, unfolds a new rich UI container displaying `tag_name`, `published_at`, and markdown `body`, finally presenting the live `Download & Install` button mapping strictly context-aware.

## Step 25: Setup Input Upgrades & Service Arrays
- [x] Swapped boolean parameters in `PanelConfig` (e.g. `HasPostgres`) for generic array structs (`Databases: []string`, `WebEngine: string`) standardizing state parsing globally across watchdog definitions.
- [x] Added integer-based list prompts during `cmd/setup/main.go` significantly improving input accuracy for firewall rules, database arrays, and explicit web engine choices.
- [x] Added "Auto Networking" workflow logic parsing FQDN internally to extract TLDs, and triggering automatic underlying DNS lookups via `net.LookupIP()` rendering subdomains dynamically without manual user intervention.
- [x] Added explicit initial firewall configuration loops (UFW / nftables) granting global HTTP/HTTPS and internal application specific bindings depending purely on dynamic configuration array parsing.

## Step 26: Setup DNS Resilience
- [x] Implemented Google DNS (IPv4 & optional IPv6) initialization directly into `/etc/resolv.conf` before `net.LookupIP` and `Certbot` execution to ensure robust resolution.
- [x] Inspected logging blocks directly in setup, verified missing logging states output correctly to console providing feedback during setup routines.

## Step 25 & 26 Re-Applied (Current Run)
- Restored `main.go` from clean backup due to previous Python patch conflicts.
- Applied exact literal matching with `replace_string_in_file` across 4 key blocks.
- **Auto Networking:** Prompt for `autoLogic` early. Evaluates root TLD structure dynamically, binds `ns1`/`ns2`, queries via `net.LookupIP` and gracefully handles fallback IPv4/IPv6 values minimizing user labor.
- **Numbered Terminal Prompts:** Implemented `promptList` for robust UI choice across Web Servers, Databases, Firewalls, and FTP mapping.
- **Google DNS Initialization Block:** Formats exactly into `/etc/resolv.conf` checking strictly for user-selected IPv6 enablement beforehand.
- All variables safely mapped to the updated `PanelConfig` payload.
- Successfully verified zero compiler errors (`go build ./cmd/setup/...` passes cleanly).

## Enable User Services & API Log Fix
- Modified `cmd/setup/main.go` to explicitly explicitly run `systemctl enable --now` for user-chosen dependencies (apache2/nginx, mariadb, postgresql, bind9, pure-ftpd, postfix, etc.) directly before starting internal daemons. This ensures critical proxy dependencies like Apache aren't dead after installation.
- Injected `GET /updates/log` directly into `internal/api/routes.go` bypassing core daemon restarts to parse `/tmp/dashboard2go-update.log`. Because decouple logic keeps NGINX/Apache alive, reading from `/tmp` natively is completely safe and properly hydrates the log window without relying on detached binaries serving API endpoints while under reboot lock.

## Networking API Adjustments & Updater Script Finalization
- Replaced the external `ifconfig.me` lookup with `api.ipify.org` and `api64.ipify.org` in `cmd/setup/main.go` for better resilience against throttling when extracting IPv4 and IPv6 respectively.
- Solidified `internal/updater/apply.go` to explicitly only restart the `dashboard2go-*` ecosystem logic while leaving `bind`, `apache`, `mariadb` completely untouched in the decoupled script. Also implemented the physical `wget` tar extraction placeholders pulling dynamically from the GitHub releases structure.

## Updater Path Architecture Repair
- Resolved critical bug where `ApplyUpdate` script would attempt to replace files globally under `/usr/local/bin` while the `cmd/setup/systemd.go` installer actually locked systemd `ExecStart` explicitly to the user's current working directory (e.g. `/root/dashBoard2go`). 
- Injected `cwd, err := os.Getwd()` directly into the bash payload generator `ApplyUpdate`. The detached update script will now physically sweep inside the correct user-deployed workspace and inject the binaries dynamically exactly where systemd is looking. 

## Setup Script Webserver Configuration & API Hardening
- [x] Injected `configureDefaultWebServer` into `cmd/setup/main.go` that directly modifies `/etc/apache2/sites-available/000-default.conf` or Nginx vhosts immediately after Let's Encrypt ACME challenge.
- [x] Mounted the actual auto-procured `live/FQDN/fullchain.pem` inside the vhost so the proxy handles SSL natively upon setup completion.
- [x] Configured native CORS overrides (`Access-Control-Allow-Origin "*"`) directly inside the generated base `<Directory>` and `location /` proxy rules to allow Javascript endpoints to pull external log files dynamically.

## Version Engine & DOM Resiliency Fixes
- [x] Changed `internal/updater/apply.go` detached bash script to launch via `systemd-run --unit=dashboard2go-update-task` instead of `nohup`. This guarantees `systemctl stop dashboard2go-core` won't recursively terminate the updater payload execution by grouping it under the same systemd cgroup context.
- [x] Altered `/updates/check` API logic to securely evaluate local sequence flags using `os.ReadFile("VERSION")` dynamically rather than relying on outdated `config.json` caches.
- [x] Patched `web/js/admin.js` to include DOM safety checks for `fetchServices()` resolving fatal network polling errors generated when updating dynamically rewrites the main UI thread.

## Final Wget & Updates Path Polish
- [x] Modified `internal/updater/apply.go` to explicitly wipe log payloads (`> $LOG_TMP` and `> $LOG_PUBLIC`) immediately cleanly initializing outputs instead of concatenating infinite logs across updates.
- [x] Added strict `v%s` tag string injection in `apply.go`'s `wget` extraction correcting GitHub's download URL payload requirements and returning HTTP 200 via `Wget`. 
- [x] Guarded `fetchUpdates` logic inside `web/js/admin.js` specifically catching inner DOM elements ensuring frontend Javascript threads don't crash loops. 

## Final Wget & Updates Path Polish
- [x] Refactored `internal/updater/check.go` and `internal/updater/apply.go` to explicitly consume `conf.UpdaterEndpoint` avoiding hardcoded GitHub URL strings and respecting user modifications.
- [x] Switched `ApplyUpdate` to natively perform a `client.Get` on the GitHub API fetching the structured `.json` payload dynamically injecting `browser_download_url`.
- [x] Updated Detached Bash Payload in `ApplyUpdate` logic dynamically executing `unzip -q` if the `.assets[0].name` payload structure reveals a `.zip` artifact instead of `.tar.gz`.
- [x] Injected an asynchronous hourly goroutine inside `cmd/core/main.go` firing `updater.CheckForUpdates(db, currentVer, conf.UpdaterEndpoint)` ensuring the Sidebar UI `data.update_available` stays strictly synchronous dynamically rather than displaying outdated smaller versions.
