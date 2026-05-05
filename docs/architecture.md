# Architecture Outline

## System Components
1. **Core API Server (`/cmd/core`)**: Bound to port 8080 (for now) acting as the single source of truth HTTP JSON API server, interfacing natively with the Web Panel.
2. **Setup CLI (`/cmd/setup`)**: An ISPConfig-inspired interactive bootstrap wizard that prepares the host OS natively to accept the control panel rules.
3. **Guard / Watchdog (`/cmd/watchdog`)**: Standalone binary that acts as a failsafe, continuously polling `systemctl` statuses and restarting core services (Nginx, MariaDB) if they drift out of line.
4. **Internal OS Wrappers (`/internal/oswrap`)**: Go functions leveraging `os/exec` directly executing host `apt-get` and `systemctl` commands safely.

## Database Philosophy
User database logic runs within the user-installed `MariaDB` or `Postgres` engines. The Panel ITSELF utilizes an embedded SQLite file to map OS-users, panel passwords, and module locations, removing the necessity of maintaining SQL connections just to render the admin screen.

## Web Server Philosophy (Strategy Pattern)
To ensure we can support both Apache and Nginx transparently:
- A generic `WebServer` Go interface is defined.
- Endpoints issue commands like `webHandler.CreateVhost(domain)`
- The installer assigns either `&ApacheWrapper{}` or `&NginxWrapper{}` to this handler depending on initial choice.

## Documentation Reference
Refer to the `plan.md` file in the project root to map exactly what phase is currently in progress.