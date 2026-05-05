.PHONY: build package clean

build:
	@echo "Building binaries for target VPS (Linux amd64)..."
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/dashboard-setup ./cmd/setup
	GOOS=linux GOARCH=amd64 go build -o dist/dashboard-core ./cmd/core
	GOOS=linux GOARCH=amd64 go build -o dist/dashboard-watchdog ./cmd/watchdog

package: build
	@echo "Packaging web assets and installation script..."
	cp -r web dist/web
	cp install.sh dist/install.sh
	@echo "Creating deployment tarball..."
	tar -czvf dashboard2go.tar.gz -C dist .
	@echo "Done! You can now SCP 'dashboard2go.tar.gz' to your test VPS."

clean:
	rm -rf dist dashboard2go.tar.gz
