# dashBoard2go Development Rules

## 1. The Strict TODO Requirement
- Every time a **mock function**, a **mocked front-end UI event** (like an HTML alert mapped over a form), or a **placeholder API route** is added *anywhere* in the codebase, a complete `// TODO: [Description of missing logic/backend requirements]` **must** be added inline next to it. 
- This ensures all needed architectural information is present and nothing is forgotten during implementation phases.

## 2. Core Architecture
- **Language Stack**: Go for the Backend (JSON API over HTTP), Vanilla JavaScript + HTML + Bootstrap 5 (Dark theme) for the Frontend. No heavy frontend frameworks.
- **Micro-service Splits**:
  - `dashboard2go-core`: Main API / UI Web Server. Fast, synchronous.
  - `dashboard2go-worker`: Asynchronous job executor pulling from SQLite queue.
  - `dashboard2go-watchdog`: Health checker and rule validator.
  - `dashboard2go-updater`: Updates, migrations, and process restarter.
- **Embedded Database Layer**: SQLite3 globally attached via `PRAGMA journal_mode=WAL;` to allow multi-process reads/writes smoothly. No MySQL dependency needed to operate the core panel logic.

## 3. Deployment & Wrappers
- **Zero-Friction Installs**: The setup script must dynamically resolve DNS (e.g. Bind9 microzones) locally if necessary to prevent Certbot validation timeouts during base deployments.
- **Service Interfaces (Wrappers)**: ALWAYS use a strict Go Interface "Strategy Pattern" for system services, separated into discrete domain packages inside `internal/wrappers/*` (e.g., `internal/wrappers/webserver`, `internal/wrappers/database`, `internal/wrappers/ssl`). Example: `webserver.WebServer` interface gracefully maps to both `webserver.ApacheWrapper` and `webserver.NginxWrapper`.
- **System Internals**: Interaction with OS-level execution MUST be handled via `internal/oswrap` utilizing raw `exec.Command` mapped strictly to Linux (Debian 12/13 primary target).

## 4. Updates & Database Migrations
- **Schema Management**: SQL definitions MUST use the `go:embed` compiler trick in `/internal/db/migrations/*.sql`. The updater automatically reads and processes `.sql` tags based on version numbers inside ACID transactions. NEVER create remote `.sql` directories for the updater to download separately.
- **Update Checks**: Updater uses standard Github API polling `repos/sickplanet/dashBoard2go/releases/latest` to validate tags against the embedded `VERSION` file. Do not push updates blindly without Admin execution approval via the UI.

## 5. System Operations & Context Security
- **Asynchronous Execution Bounds**: OS-level package operations (e.g., APT actions) or long-running daemon restarts MUST be executed with an explicit `context.Context` timeout or offloaded to the asynchronous `dashboard2go-worker` queue. This prevents the REST API from blocking indefinitely and HTTP timeouts during slow mirrors, ensuring unresponsive native OS apps don't bring down the main dashboard interface.

## 6. Self-Updating Protocol
- **Updating the Updater**: Binaries cannot overwrite themselves seamlessly under `systemd` due to active file locks (`Text file busy`). 
- **Decoupled execution**: Any structural deployment replacing `/usr/local/bin/dashboard2go-*` binaries MUST drop an asynchronous, fully detached shell script (e.g. `/tmp/apply-update.sh`) executing via `nohup ... &`.
- **Pre-execution constraints**: The detached script assumes total control, explicitly executing `systemctl stop dashboard2go-*` for all ecosystem services (core, watchdog, worker, updater), cleanly executing `rm -f /usr/local/bin/dashboard2go-*`, extracting the github payloads, and triggering `systemctl start`.

## 7. REST API Semantics & Frontend Bindings
- **Strict HTTP Methods**: All API endpoints MUST adhere strictly to RESTful conventions throughout the Golang backend and JavaScript frontend. 
  - `GET`: Information retrieval and telemetry fetching.
  - `POST`: Resource creation, initiating actions, and triggering background worker jobs.
  - `PUT`/`PATCH`: Modifying existing configurations or toggling states (e.g., firewall/daemon toggles).
  - `DELETE`: Removing resources.
- **Frontend Consistency**: Never use raw `fetch()` calls in the frontend UI. All frontend-to-backend integrations must strictly utilize the generic `apiFetch` wrapper mapping (defined in `web/js/core.js`) to guarantee uniform JSON headers, correct method dispatch, and consistent asynchronous error state unwrapping across the entire dashboard interface.

## 8. Git Workflow & Version Control
- **NEVER EVER PUSH DIRECTLY TO MAIN.**
- All commits and feature branches MUST be pushed strictly to `dev`.
- Post-validation, `dev` is then merged with `main` to ensure a clean, linear, and easily reviewable git history. Do not bypass the `dev` branch under any circumstances.
