# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.0.0] - 2026-05-05

### Added
- **Interactive Setup (`dashboard2go-setup`)**: Headless bootstrapper that configures UFW firewall, requests user stack preferences, provisions admin passwords, and auto-generates `systemd` services natively.
- **Worker Queue System (`dashboard2go-worker`)**: Decoupled asynchronous processing utilizing SQLite WAL mode to execute background deployments safely (vhosts, panel users, FTP).
- **Watchdog & Monitoring (`dashboard2go-watchdog`)**: Persistent daemon that scans user-specific `error.log` files and generates UI alerts based on user SQLite preferences.
- **Strict Directory Architecture**: Built ISPConfig-style sandboxes globally structured at `/home/dashboard2go/users/{username}/`.
- **PHP Integration**: Deep integration with PHP 8.3-FPM (`pm = ondemand`), creating native user sockets and strictly locking `open_basedir` & `session.save_path`.
- **Service Wrappers**: Fully abstracted Go struct execution paths for Nginx, Apache2, PureFTPd, Postfix, Dovecot, MariaDB, and Bind9.
- **Core API Server (`dashboard2go-core`)**: SNI-aware HTTP/HTTPS proxy serving both encrypted and standard endpoints concurrently. 
- **Web UI & Dashboard**: Responsive Admin & User HTML/CSS layouts utilizing Javascript `fetch()` APIs talking directly to Go endpoints for Live Status polling and payload execution.
- **CI/CD Pipeline**: GitHub Actions `.github/workflows/release.yml` workflows establishing automated compilation, `.zip` packaging, and version bump validations for PRs matching `/VERSION`.

