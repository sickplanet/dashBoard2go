#!/bin/bash
# dashBoard2go Installer Script
# Inspiration: cPanel / ISPConfig
# Target: Fresh Debian / Ubuntu Installations
# Must run as root

set -e

echo "==================================================="
echo "  dashBoard2go - Web Hosting Control Panel Setup   "
echo "==================================================="

if [ "$EUID" -ne 0 ]; then
  echo "Please run this script as root."
  exit 1
fi

echo "[1/4] Updating system packages..."
apt-get update -y
apt-get install -y wget curl git

echo "[2/5] Preparing DashBoard2Go Core Binaries..."
# Ensure Go is present
if ! command -v go &> /dev/null; then
  echo "Installing Go compiler..."
  apt-get install -y golang
fi

echo "Building core suite from source..."
go mod tidy
go build -o /usr/local/bin/dashboard2go-core ./cmd/core
go build -o /usr/local/bin/dashboard2go-worker ./cmd/worker
go build -o /usr/local/bin/dashboard2go-watchdog ./cmd/watchdog
go build -o /usr/local/bin/dashboard2go-setup ./cmd/setup

echo "[3/5] Creating UI Directories..."
mkdir -p /var/www/dashboard2go
# Assuming we are running this from the source directory directly
if [ -d "./web" ]; then
  cp -r ./web /usr/local/bin/
fi

echo "[4/5] Launching Interactive Setup..."
cd /usr/local/bin
/usr/local/bin/dashboard2go-setup

echo "[5/5] Initialization script finished."
