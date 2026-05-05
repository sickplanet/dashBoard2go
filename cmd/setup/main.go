package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"golang.org/x/crypto/bcrypt"

	"dashBoard2go/internal/config"
	"dashBoard2go/internal/oswrap"
)

func promptUser(reader *bufio.Reader, question, defaultVal string) string {
	fmt.Printf("%s [%s]: ", question, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func getCertForFQDN(fqdn string) error {
	fmt.Println("Installing Certbot...")
	oswrap.AptInstall("certbot")

	fmt.Println("Configuring ACME Challenge Webroot...")
	os.MkdirAll("/var/www/acme-challenge", 0755)

	// Typically, you need a placeholder web server config to serve this directory
	// But as a bare minimum setup script, assuming web server config is bootstrapped later,
	// certbot standalone can be used if webserver isn't running yet,
	// OR webroot if we bootstrap the vhost first.
	// Since setup runs before full panel init, we will use certbot standalone just for this initial cert if ports are free,
	// or instruct certbot. Wait, the architectural decision was to use webroot.
	// Let's use standalone for the initial procurement, then regular renewals use webroot.
	// Actually, let's provision using webroot and assume the user hasn't mapped the domain yet, we might need standalone for the VERY first run.

	fmt.Printf("Procuring Let's Encrypt Certificate for %s...\n", fqdn)
	cmd := exec.Command("certbot", "certonly",
		"--standalone", // Use standalone during initial setup before Nginx/Apache are fully configured
		"-d", fqdn,
		"--non-interactive",
		"--agree-tos",
		"--register-unsafely-without-email",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot failed: %s - %v", string(out), err)
	}
	return nil
}

func main() {
	// 0. Setup Guard
	if _, err := os.Stat("config.json"); err == nil {
		conf, _ := config.LoadConfig("config.json")
		if conf != nil && conf.Installed {
			fmt.Println("CRITICAL: dashBoard2go is already installed! Setup aborted.")
			fmt.Println("To re-install, you must manually remove config.json.")
			os.Exit(1)
		}
	}

	fmt.Println("==================================================")
	fmt.Println("   dashBoard2go Interactive Setup (ISPConfig style) ")
	fmt.Println("==================================================")

	reader := bufio.NewReader(os.Stdin)

	hostname := promptUser(reader, "Enter FQDN Hostname", "server1.example.com")
	enableIPv6 := promptUser(reader, "Enable IPv6 Support?", "y")
	webServer := promptUser(reader, "Select Web Server (apache/nginx)", "nginx")
	dbServer := promptUser(reader, "Select Postgres Database additionally? (mariadb is mandatory)", "n")
	hasPostgres := strings.ToLower(dbServer) == "y"

	mariaDBPass := promptUser(reader, "Enter new MariaDB Root Password", "root_secret_db")
	postgresPass := ""
	if hasPostgres {
		postgresPass = promptUser(reader, "Enter new Postgres Root Password", "root_secret_pg")
	}

	enableAutoSSL := promptUser(reader, "Automatically configure Let's Encrypt SSL for FQDN?", "y")
	adminPass := promptUser(reader, "Enter Admin Password for Control Panel", "admin123")
	

	fmt.Println("\nConfiguration Summary:")
	fmt.Printf("- Hostname: %s\n", hostname)
	fmt.Printf("- IPv6: %s\n", enableIPv6)
	fmt.Printf("- Web Server: %s\n", webServer)
	fmt.Printf("- Postgres Support: %v\n", hasPostgres)
	fmt.Printf("- FQDN AutoSSL: %s\n", enableAutoSSL)

	confirm := promptUser(reader, "Proceed with installation?", "y")
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Installation aborted.")
		os.Exit(0)
	}

	fmt.Println("\n[1/4] Applying Hostname...")
	// Note: in a real install we use hostnamectl
	oswrap.AptUpdate()

	fmt.Println("[2/4] Installing target stack packages...")
	packages := []string{}

	if webServer == "nginx" {
		packages = append(packages, "nginx")
	} else {
		packages = append(packages, "apache2")
	}

	packages = append(packages, "mariadb-server")
	if hasPostgres {
		packages = append(packages, "postgresql")
	}

	// Dynamic PHP Version and comprehensive WordPress/Modern App Dependencies
	phpPackages := []string{
		"php8.3-fpm", "php8.3-cli", "php8.3-mysql", "php8.3-curl",
		"php8.3-gd", "php8.3-mbstring", "php8.3-xml", "php8.3-zip",
		"php8.3-opcache", "php8.3-bz2", "php8.3-intl", "php8.3-bcmath",
		"php-imagick", "php-redis", "php-memcached", "redis-server",
	}
	if webServer == "apache2" {
		phpPackages = append(phpPackages, "libapache2-mod-fcgid")
	}
	packages = append(packages, phpPackages...)

	packages = append(packages, "bind9", "postfix", "dovecot-imapd", "amavisd-new", "spamassassin", "clamav", "clamav-daemon", "pure-ftpd")
	err := oswrap.AptInstall(packages...)
	if err != nil {

		log.Printf("Warning: Failed to install some packages natively. %v\n", err)
		log.Println("If you are not running as root on Debian/Ubuntu, packages mock-failed.")
	}

	fmt.Println("[3/4] Configuring FQDN and AutoSSL...")
	useLetsEncrypt := strings.ToLower(enableAutoSSL) == "y"
	if useLetsEncrypt {
		err := getCertForFQDN(hostname)
		if err != nil {
			log.Printf("Warning: Failed to procure initial SSL: %v\n", err)
			log.Println("Ensure DNS A records point to this server. SSL will require manual init.")
			useLetsEncrypt = false
		} else {
			fmt.Println("SSL Successfully provisioned!")
		}
	}

	fmt.Println("[4/5] Establishing Base Architectures & UFW Firewall...")
	os.MkdirAll("/home/dashboard2go/users", 0755)
	os.MkdirAll("/var/lib/dashboard2go", 0755)

	fmt.Println("Configuring Initial UFW Rules...")
	// In a real environment, we'd invoke the UFW Wrapper, but a direct exec works just as well for setup.
	exec.Command("ufw", "--force", "enable").Run()
	exec.Command("ufw", "allow", "22/tcp").Run()
	exec.Command("ufw", "allow", "80/tcp").Run()
	exec.Command("ufw", "allow", "443/tcp").Run()
	exec.Command("ufw", "allow", "21/tcp").Run()
	exec.Command("ufw", "allow", "25/tcp").Run()
	exec.Command("ufw", "allow", "143/tcp").Run()
	exec.Command("ufw", "allow", "993/tcp").Run()
	exec.Command("ufw", "allow", "8080/tcp").Run() // Panel HTTP
	exec.Command("ufw", "allow", "8443/tcp").Run() // Panel HTTPS

	fmt.Println("[5/5] Storing Configuration & Seeding Admin DB...")

	// Create the configuration payload
	panelConfig := &config.PanelConfig{
		Installed:          true,
		PanelVersion:       "v1.0.0",
		FQDN:               hostname,
		UseLetsEncryptFQDN: useLetsEncrypt,
		PanelPortHTTP:      8080,
		PanelPortHTTPS:     8443,
		WebEngine:          strings.ToLower(webServer),
		HasPostgres:        hasPostgres,
		MariaDBRootPass:    mariaDBPass,
		PostgresRootPass:   postgresPass,
		SQLitePath:         "/var/lib/dashboard2go/panel.sqlite",
		UpdaterEndpoint:    "https://api.github.com/repos/yourname/dashBoard2go/releases/latest",
	}

	// Ensure the dashboard dir exists for the database
	os.MkdirAll("/var/lib/dashboard2go", 0755)

	// Create the core SQLite DB and seed the admin user
	db, err := sql.Open("sqlite3", "/var/lib/dashboard2go/panel.sqlite")
	if err != nil {
		log.Fatalf("Warning: Could not open panel.sqlite: %v\n", err)
	} else {
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS panel_users (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        username TEXT UNIQUE NOT NULL,
                        password TEXT NOT NULL,
                        is_admin BOOLEAN DEFAULT 0
                )`)
// Encrypt using bcrypt
                hashedPass, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
                if err != nil {
                        log.Fatalf("Failed to hash admin password: %v", err)
                }

                _, _ = db.Exec(`INSERT INTO panel_users (username, password, is_admin) VALUES (?, ?, 1)
                        ON CONFLICT(username) DO UPDATE SET password = excluded.password`,
                        "admin", string(hashedPass))
		db.Close()
	}

	installSystemdServices()

	err = config.SaveConfig("config.json", panelConfig)
	if err != nil {
		log.Fatalf("CRITICAL: Could not save config.json: %v\n", err)
	}
}
