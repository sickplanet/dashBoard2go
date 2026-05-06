package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"golang.org/x/crypto/bcrypt"

	"dashBoard2go/internal/config"
	"dashBoard2go/internal/oswrap"
	"dashBoard2go/internal/wrappers/dns"
	"dashBoard2go/internal/wrappers/ssl"
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

// getPublicIP fetches the external IP
func getPublicIP() string {
	cmd := exec.Command("curl", "-s", "https://ifconfig.me")
	out, err := cmd.Output()
	if err != nil {
		return "127.0.0.1"
	}
	return string(out)
}

// setupLocalBindZone creates an authoritative zone for the FQDN
// createMicroZone binds an explicit exact-match domain to this server IP natively.
func createMicroZone(domain, ns1, ns2, ip string) {
	if domain == "" {
		return
	}

	// Ensure the dashboard configuration is included globally
	namedLocal, _ := os.ReadFile("/etc/bind/named.conf.local")
	if !strings.Contains(string(namedLocal), "/etc/bind/named.conf.dashboard") {
		f, err := os.OpenFile("/etc/bind/named.conf.local", os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString("\ninclude \"/etc/bind/named.conf.dashboard\";\n")
			f.Close()
		}
	}

	b9 := dns.NewBind9Wrapper()
	cfg := dns.ZoneConfig{
		Domain:     domain,
		AdminEmail: "admin." + domain,
		PrimaryIP:  ip,
		Nameserver: ns1,
	}

	// 1. Create the base zone & entry
	_ = b9.CreateZone(cfg)

	// 2. Add extra NS records explicitly via our wrapper
	if ns2 != "" && ns1 != ns2 {
		b9.AddRecord(domain, dns.DNSRecord{
			Type:  "NS",
			Name:  "@",
			Value: ns2 + ".",
			TTL:   86400,
		})
	}
}

// setupLocalBindZone creates primarily ONE authoritative zone for the Base Domain
// and populates it with A records for the FQDN and Nameservers to prevent CAA lookup loops and BIND zone conflicts.
func setupLocalBindZone(fqdn, baseDomain, ns1, ns2, ip1, ip2 string) {
	fmt.Println("Configuring single authoritative local Bind9 Zone for Base Domain...")

	// Strictly limit zones to base domains. Do NOT create microzones for ns1/ns2/subdomains natively.
	createMicroZone(baseDomain, ns1, ns2, ip1)

	b9 := dns.NewBind9Wrapper()

	// Helper to safely strip suffix and add an A record
	addSubdomainRecord := func(fullDomain, ip string) {
		fullDomain = strings.TrimSpace(fullDomain)
		if fullDomain == baseDomain || fullDomain == "" {
			return
		}

		importStr := "." + baseDomain
		if strings.HasSuffix(fullDomain, importStr) {
			sub := strings.TrimSuffix(fullDomain, importStr)
			if sub != "" {
				b9.AddRecord(baseDomain, dns.DNSRecord{
					Type:  "A",
					Name:  sub,
					Value: ip,
					TTL:   86400,
				})
			}
		}
	}

	addSubdomainRecord(fqdn, ip1)
	addSubdomainRecord(ns1, ip1)

	if ip2 != "" {
		addSubdomainRecord(ns2, ip2)
	} else {
		addSubdomainRecord(ns2, ip1)
	}

	// Ensure BIND listens publicly and allows external queries so Let's Encrypt can resolve us
	optionsContent := `options {
	directory "/var/cache/bind";
	dnssec-validation auto;
	listen-on-v6 { any; };
	listen-on { any; };
	allow-query { any; };
};
`
	os.WriteFile("/etc/bind/named.conf.options", []byte(optionsContent), 0644)

	exec.Command("systemctl", "restart", "bind9").Run()
	exec.Command("systemctl", "restart", "named").Run() // Redhat/Alma fallback name
	fmt.Println("Bind9 explicit micro-zones loaded successfully.")
}

func getCertForFQDN(fqdn string) error {
	fmt.Printf("Pre-flight Global DNS Resolution check for %s...\n", fqdn)
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.Dial("udp", "8.8.8.8:53")
		},
	}

	// Fast lookup via Google DNS to confirm domain is active globally.
	ips, err := r.LookupIPAddr(context.Background(), fqdn)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("WARNING: Global DNS resolution (Google 8.8.8.8) failed for %s. The domain is not globally propagated or valid yet: %v", fqdn, err)
	}

	fmt.Printf("Global DNS validated. %s resolved to %v\n", fqdn, ips)

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

	// Stop web services so certbot can bind port 80
	exec.Command("systemctl", "stop", "nginx").Run()
	exec.Command("systemctl", "stop", "apache2").Run()

	certManager := ssl.NewCertbotManager("standalone")
	err = certManager.ObtainCertStandalone(context.Background(), fqdn)
	if err != nil {
		return fmt.Errorf("certbot wrapper failed: %v", err)
	}
	return nil
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	// 0. Setup Guard
	if _, err := os.Stat("config.json"); err == nil {
		conf, _ := config.LoadConfig("config.json")
		if conf != nil && conf.Installed {
			fmt.Println("WARNING: dashBoard2go is already installed and config.json exists.")
			fmt.Println("Development Mode active.")
			mode := promptUser(reader, "Do you want to [1] EXIT, or [2] FIX BROKEN (Overwrite configs)?", "1")
			if mode != "2" {
				fmt.Println("Setup aborted.")
				os.Exit(0)
			}
			fmt.Println("Proceeding with FIX BROKEN mode. Configurations will be overwritten...")
		}
	}

	fmt.Println("==================================================")
	fmt.Println("   dashBoard2go Interactive Setup (ISPConfig style) ")
	fmt.Println("==================================================")

	hostname := promptUser(reader, "Enter FQDN Hostname", "server1.example.com")
	baseDomain := promptUser(reader, "Enter Base Domain Name (e.g. domain.ro / example.com)", "example.com")
	ns1 := promptUser(reader, "Enter Primary Nameserver", "ns1.example.com")
	ns2 := promptUser(reader, "Enter Secondary Nameserver", "ns2.example.com")
	detectedIP := getPublicIP()
	serverIP1 := promptUser(reader, "Enter Primary Server IP (for FQDN & NS1)", detectedIP)
	serverIP2 := promptUser(reader, "Enter Secondary Server IP (for NS2, leave blank if same as IP1)", serverIP1)
	if serverIP2 == "" {
		serverIP2 = serverIP1
	}
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

	fmt.Println("\n[1/5] Applying Hostname...")
	exec.Command("hostnamectl", "set-hostname", hostname).Run()
	os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644)
	shortHostname := strings.Split(hostname, ".")[0]
	hostsEntry := fmt.Sprintf("127.0.1.1\t%s\t%s\n", hostname, shortHostname)
	hostsFile, _ := os.ReadFile("/etc/hosts")
	if !strings.Contains(string(hostsFile), hostname) {
		f, _ := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, 0600)
		if f != nil {
			f.WriteString(hostsEntry)
			f.Close()
		}
	}

	oswrap.AptUpdate()

	fmt.Println("[2/5] Installing target stack packages...")
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

	// PHP Dependencies (using OS default PHP version)
	phpPackages := []string{
		"php-fpm", "php-cli", "php-mysql", "php-curl",
		"php-gd", "php-mbstring", "php-xml", "php-zip",
		"php-bz2", "php-intl", "php-bcmath", "php-common",
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

	fmt.Println("Configuring Initial UFW Rules...")
	// In a real environment, we'd invoke the UFW Wrapper, but a direct exec works just as well for setup.
	exec.Command("ufw", "--force", "enable").Run()
	exec.Command("ufw", "allow", "22/tcp").Run()
	exec.Command("ufw", "allow", "53/tcp").Run() // DNS
	exec.Command("ufw", "allow", "53/udp").Run() // DNS
	exec.Command("ufw", "allow", "80/tcp").Run()
	exec.Command("ufw", "allow", "443/tcp").Run()
	exec.Command("ufw", "allow", "21/tcp").Run()
	exec.Command("ufw", "allow", "25/tcp").Run()   // SMTP
	exec.Command("ufw", "allow", "143/tcp").Run()  // IMAP
	exec.Command("ufw", "allow", "465/tcp").Run()  // SMTPS
	exec.Command("ufw", "allow", "587/tcp").Run()  // SMTP Submission
	exec.Command("ufw", "allow", "993/tcp").Run()  // IMAPS
	exec.Command("ufw", "allow", "995/tcp").Run()  // POP3S
	exec.Command("ufw", "allow", "8080/tcp").Run() // Panel HTTP
	exec.Command("ufw", "allow", "8443/tcp").Run() // Panel HTTPS

	fmt.Println("[3/5] Configuring FQDN and AutoSSL...")
	useLetsEncrypt := strings.ToLower(enableAutoSSL) == "y"

	// Create authoritative zone before probing Let's encrypt
	setupLocalBindZone(hostname, baseDomain, ns1, ns2, serverIP1, serverIP2)

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

	fmt.Println("[4/5] Establishing Base Architectures...")
	os.MkdirAll("/home/dashboard2go/users", 0755)
	os.MkdirAll("/var/lib/dashboard2go", 0755)

	fmt.Println("[5/5] Storing Configuration & Seeding Admin DB...")

	// Create the configuration payload
	panelConfig := &config.PanelConfig{
		Installed:          true,
		PanelVersion:       "v1.0.0",
		FQDN:               hostname,
		NS1:                ns1,
		NS2:                ns2,
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
