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
	"time"

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

func promptList(reader *bufio.Reader, title string, options []string) int {
	fmt.Printf("\n%s\n", title)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	for {
		fmt.Printf("Select option [1-%d]: ", len(options))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		var val int
		if _, err := fmt.Sscanf(input, "%d", &val); err == nil && val >= 1 && val <= len(options) {
			return val
		}
		fmt.Println("Invalid input, try again.")
	}
}

func resolveIP(host string) (ipv4, ipv6 string) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", ""
	}
	for _, ip := range ips {
		if ipv4 == "" && ip.To4() != nil {
			ipv4 = ip.String()
		} else if ipv6 == "" && ip.To4() == nil {
			ipv6 = ip.String()
		}
	}
	return
}

// getPublicIP fetches the external IP
func getPublicIP() string {
	cmd := exec.Command("curl", "-s", "-4", "https://api.ipify.org")
	out, err := cmd.Output()
	if err != nil {
		return "127.0.0.1"
	}
	return string(out)
}

// getPublicIPv6 fetches the external IPv6
func getPublicIPv6() string {
	cmd := exec.Command("curl", "-s", "-6", "https://api64.ipify.org")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// setupLocalBindZone creates an authoritative zone for the FQDN
// createMicroZone binds an explicit exact-match domain to this server IP natively.
func createMicroZone(domain, ns1, ns2, ip, ipv6 string) {
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

	// Add AAAA records
	if ipv6 != "" {
		b9.AddRecord(domain, dns.DNSRecord{Type: "AAAA", Name: "@", Value: ipv6, TTL: 86400})
		b9.AddRecord(domain, dns.DNSRecord{Type: "AAAA", Name: "www", Value: ipv6, TTL: 86400})
	}

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
func setupLocalBindZone(fqdn, baseDomain, ns1, ns2, ip1, ip2, ipv6_1, ipv6_2 string) {
	fmt.Println("Configuring single authoritative local Bind9 Zone for Base Domain...")

	// Strictly limit zones to base domains. Do NOT create microzones for ns1/ns2/subdomains natively.
	createMicroZone(baseDomain, ns1, ns2, ip1, ipv6_1)

	b9 := dns.NewBind9Wrapper()

	// Helper to safely strip suffix and add an A or AAAA record
	addSubdomainRecord := func(fullDomain, ip, recType string) {
		fullDomain = strings.TrimSpace(fullDomain)
		if fullDomain == baseDomain || fullDomain == "" || ip == "" {
			return
		}

		importStr := "." + baseDomain
		if strings.HasSuffix(fullDomain, importStr) {
			sub := strings.TrimSuffix(fullDomain, importStr)
			if sub != "" {
				b9.AddRecord(baseDomain, dns.DNSRecord{
					Type:  recType,
					Name:  sub,
					Value: ip,
					TTL:   86400,
				})
			}
		}
	}

	addSubdomainRecord(fqdn, ip1, "A")
	addSubdomainRecord(ns1, ip1, "A")
	if ipv6_1 != "" {
		addSubdomainRecord(fqdn, ipv6_1, "AAAA")
		addSubdomainRecord(ns1, ipv6_1, "AAAA")
	}

	if ip2 != "" {
		addSubdomainRecord(ns2, ip2, "A")
	} else {
		addSubdomainRecord(ns2, ip1, "A")
	}

	if ipv6_2 != "" {
		addSubdomainRecord(ns2, ipv6_2, "AAAA")
	} else if ipv6_1 != "" {
		addSubdomainRecord(ns2, ipv6_1, "AAAA")
	}

	// Inject the secondary nameserver as an NS record for the zone to prevent Let's Encrypt "Lame Delegation" DNS timeouts
	if ns2 != "" && ns1 != ns2 {
		b9.AddRecord(baseDomain, dns.DNSRecord{
			Type:  "NS",
			Name:  "@",
			Value: ns2 + ".",
			TTL:   86400,
		})
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

	// Force a 5-second sleep so BIND9 has time to announce UDP 53 routes and load physical zone files
	// before the certbot daemon triggers Let's Encrypt validation.
	time.Sleep(5 * time.Second)
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

	fmt.Println("Applying initial Google DNS to /etc/resolv.conf...")
	enableIPv6 := promptUser(reader, "Enable IPv6 Support? (Will also apply IPv6 Google DNS)", "y")
	useIPv6 := strings.ToLower(enableIPv6) == "y"

	resolvConf := "nameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if useIPv6 {
		resolvConf += "nameserver 2001:4860:4860::8888\nnameserver 2001:4860:4860::8844\n"
	}
	os.WriteFile("/etc/resolv.conf", []byte(resolvConf), 0644)
	fmt.Println("Initial DNS applied successfully.")

	hostname := promptUser(reader, "Enter FQDN Hostname", "server1.example.com")
	autoLogic := promptUser(reader, "Use default auto logic for networking? (Extract domain from FQDN, use TLD info)", "y")

	var baseDomain, ns1, ns2, serverIP1, serverIP2, serverIPv6_1, serverIPv6_2 string

	if strings.ToLower(autoLogic) == "y" {
		parts := strings.Split(hostname, ".")
		if len(parts) >= 2 {
			baseDomain = strings.Join(parts[len(parts)-2:], ".")
		} else {
			baseDomain = hostname
		}
		ns1 = "ns1." + baseDomain
		ns2 = "ns2." + baseDomain

		fmt.Printf("Auto-extracted Base Domain: %s\n", baseDomain)
		fmt.Printf("Auto-generated NS1: %s\n", ns1)
		fmt.Printf("Auto-generated NS2: %s\n", ns2)

		v4_1, v6_1 := resolveIP(ns1)
		v4_2, v6_2 := resolveIP(ns2)

		serverIP1 = v4_1
		serverIP2 = v4_2
		if serverIP1 == "" {
			serverIP1 = getPublicIP()
		}
		if serverIP2 == "" {
			serverIP2 = serverIP1
		}

		serverIPv6_1 = v6_1
		serverIPv6_2 = v6_2
		if serverIPv6_1 != "" {
			useIPv6 = true
			enableIPv6 = "y"
		}
		if serverIPv6_2 == "" {
			serverIPv6_2 = serverIPv6_1
		}
		fmt.Printf("Resolved IP1: %s, IP2: %s\n", serverIP1, serverIP2)
		if useIPv6 {
			fmt.Printf("Resolved IPv6_1: %s, IPv6_2: %s\n", serverIPv6_1, serverIPv6_2)
		}
	} else {
		baseDomain = promptUser(reader, "Enter Base Domain Name (e.g. domain.ro / example.com)", "example.com")
		ns1 = promptUser(reader, "Enter Primary Nameserver", "ns1.example.com")
		ns2 = promptUser(reader, "Enter Secondary Nameserver", "ns2.example.com")
		detectedIP := getPublicIP()
		serverIP1 = promptUser(reader, "Enter Primary Server IP (for FQDN & NS1)", detectedIP)
		serverIP2 = promptUser(reader, "Enter Secondary Server IP (for NS2, leave blank if same as IP1)", serverIP1)
		if serverIP2 == "" {
			serverIP2 = serverIP1
		}

		if useIPv6 {
			detectedIPv6 := getPublicIPv6()
			serverIPv6_1 = promptUser(reader, "Enter Primary Server IPv6 (for FQDN & NS1)", detectedIPv6)
			serverIPv6_2 = promptUser(reader, "Enter Secondary Server IPv6 (for NS2, leave blank if same as IPv6_1)", serverIPv6_1)
			if serverIPv6_2 == "" {
				serverIPv6_2 = serverIPv6_1
			}
		}
	}

	webEngineChoices := []string{"Apache2", "Nginx"}
	wsIdx := promptList(reader, "Select Web Server Engine:", webEngineChoices)
	webServer := "apache2"
	if wsIdx == 2 {
		webServer = "nginx"
	}

	dbChoices := []string{"MariaDB Only", "MariaDB + Postgres"}
	dbIdx := promptList(reader, "Select Database Stack:", dbChoices)
	hasPostgres := (dbIdx == 2)

	fwChoices := []string{"None (External/LAN)", "UFW", "nftables"}
	fwIdx := promptList(reader, "Select Firewall:", fwChoices)
	firewallChoice := "none"
	if fwIdx == 2 {
		firewallChoice = "ufw"
	} else if fwIdx == 3 {
		firewallChoice = "nftables"
	}

	ftpChoices := []string{"None", "Pure-FTPd"}
	ftpIdx := promptList(reader, "Select FTP Server:", ftpChoices)
	ftpChoice := "none"
	if ftpIdx == 2 {
		ftpChoice = "pure-ftpd"
	}

	mariaDBPass := promptUser(reader, "Enter new MariaDB Root Password", "root_secret_db")
	postgresPass := ""
	if hasPostgres {
		postgresPass = promptUser(reader, "Enter new Postgres Root Password", "root_secret_pg")
	}

	enableAutoSSL := promptUser(reader, "Automatically configure Let's Encrypt SSL for FQDN?", "y")
	adminPass := promptUser(reader, "Enter Admin Password for Control Panel", "admin123")

	fmt.Println("\nConfiguration Summary:")
	fmt.Printf("- Hostname: %s\n", hostname)
	fmt.Printf("- Base Domain: %s\n", baseDomain)
	fmt.Printf("- NS1: %s, NS2: %s\n", ns1, ns2)
	fmt.Printf("- IPv4: %s\n", serverIP1)
	if useIPv6 {
		fmt.Printf("- IPv6: %s\n", serverIPv6_1)
	}
	fmt.Printf("- Web Server: %s\n", webServer)
	fmt.Printf("- Postgres Support: %v\n", hasPostgres)
	fmt.Printf("- Firewall: %s\n", firewallChoice)
	fmt.Printf("- FTP Server: %s\n", ftpChoice)
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

	packages = append(packages, "bind9", "postfix", "dovecot-imapd", "amavisd-new", "spamassassin", "clamav", "clamav-daemon")

	if ftpChoice == "pure-ftpd" {
		packages = append(packages, "pure-ftpd")
	}

	if firewallChoice == "ufw" {
		packages = append(packages, "ufw")
	} else if firewallChoice == "nftables" {
		packages = append(packages, "nftables")
	}

	err := oswrap.AptInstall(packages...)
	if err != nil {

		log.Printf("Warning: Failed to install some packages natively. %v\n", err)
		log.Println("If you are not running as root on Debian/Ubuntu, packages mock-failed.")
	}

	if firewallChoice != "none" {
		fmt.Printf("Configuring Initial %s Rules...\n", firewallChoice)
		if firewallChoice == "ufw" {
			exec.Command("ufw", "--force", "enable").Run()
			exec.Command("ufw", "allow", "22/tcp").Run()  // SSH
			exec.Command("ufw", "allow", "53/tcp").Run()  // DNS
			exec.Command("ufw", "allow", "53/udp").Run()  // DNS
			exec.Command("ufw", "allow", "80/tcp").Run()  // HTTP
			exec.Command("ufw", "allow", "443/tcp").Run() // Allow HTTP/HTTPS
			if ftpChoice == "pure-ftpd" {
				exec.Command("ufw", "allow", "21/tcp").Run() // FTP
			}
			exec.Command("ufw", "allow", "25/tcp").Run()   // SMTP
			exec.Command("ufw", "allow", "143/tcp").Run()  // IMAP
			exec.Command("ufw", "allow", "465/tcp").Run()  // SMTPS
			exec.Command("ufw", "allow", "587/tcp").Run()  // SMTP Submission
			exec.Command("ufw", "allow", "993/tcp").Run()  // IMAPS
			exec.Command("ufw", "allow", "995/tcp").Run()  // POP3S
			exec.Command("ufw", "allow", "8080/tcp").Run() // Panel HTTP
			exec.Command("ufw", "allow", "8443/tcp").Run() // Panel HTTPS
		} else if firewallChoice == "nftables" {
			log.Println("Note: basic nftables rules must be applied manually or via wrapper. Enabling service...")
			exec.Command("systemctl", "enable", "--now", "nftables").Run()
		}
	}

	fmt.Println("[3/5] Configuring FQDN and AutoSSL...")
	useLetsEncrypt := strings.ToLower(enableAutoSSL) == "y"

	// Create authoritative zone before probing Let's encrypt
	setupLocalBindZone(hostname, baseDomain, ns1, ns2, serverIP1, serverIP2, serverIPv6_1, serverIPv6_2)

	// Ensure BIND9 actually reloads the zone file and has time to bind UDP port 53 before Let's Encrypt calls
	fmt.Println("Reloading BIND9 and waiting 5 seconds for DNS propagation over localhost...")
	exec.Command("systemctl", "restart", "bind9").Run()
	time.Sleep(5 * time.Second)

	if !useLetsEncrypt {
		configureDefaultWebServer(webServer, hostname, false)
	}

	if useLetsEncrypt {
		err := getCertForFQDN(hostname)
		if err != nil {
			log.Printf("Warning: Failed to procure initial SSL: %v\n", err)
			log.Println("Ensure DNS A records point to this server. SSL will require manual init.")
			configureDefaultWebServer(webServer, hostname, false)
			useLetsEncrypt = false
		} else {
			fmt.Println("SSL Successfully provisioned!")
			configureDefaultWebServer(webServer, hostname, useLetsEncrypt)
		}
	}

	fmt.Println("[4/5] Establishing Base Architectures...")
	os.MkdirAll("/home/dashboard2go/users", 0755)
	os.MkdirAll("/var/lib/dashboard2go", 0755)

	fmt.Println("[5/5] Storing Configuration & Seeding Admin DB...")

	dbList := []string{"mariadb"}
	if hasPostgres {
		dbList = append(dbList, "postgresql")
	}

	webEngineSafe := strings.ToLower(webServer)
	if webEngineSafe == "apache" {
		webEngineSafe = "apache2"
	}

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
		WebEngine:          webEngineSafe,
		Databases:          dbList,
		DNSServer:          "bind9",
		Firewall:           firewallChoice,
		FTPServer:          ftpChoice,
		MariaDBRootPass:    mariaDBPass,
		PostgresRootPass:   postgresPass,
		SQLitePath:         "/var/lib/dashboard2go/panel.sqlite",
		UpdaterEndpoint:    "https://api.github.com/repos/sickplanet/dashBoard2go/releases/latest",
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

	err = config.SaveConfig("config.json", panelConfig)
	if err != nil {
		log.Fatalf("CRITICAL: Could not save config.json: %v\n", err)
	}

	fmt.Println("Enabling and starting user-selected services...")
	servicesToStart := []string{"bind9", "postfix", "dovecot"}
	servicesToStart = append(servicesToStart, webServer)
	servicesToStart = append(servicesToStart, "mariadb")
	if hasPostgres {
		servicesToStart = append(servicesToStart, "postgresql")
	}
	if ftpChoice == "pure-ftpd" {
		servicesToStart = append(servicesToStart, "pure-ftpd")
	}
	for _, svc := range servicesToStart {
		exec.Command("systemctl", "enable", "--now", svc).Run()
	}

	installSystemdServices()
}

func configureDefaultWebServer(webServer, fqdn string, useSSL bool) {
	if webServer == "apache2" {
		fmt.Println("Configuring default Apache2 vhost...")
		exec.Command("a2enmod", "ssl").Run()
		exec.Command("a2enmod", "headers").Run()

		var vhostContent string
		if useSSL {
			vhostContent = fmt.Sprintf(`<VirtualHost *:80>
ServerName %s
DocumentRoot /var/www/html
Redirect permanent / https://%s/
</VirtualHost>

<VirtualHost *:443>
ServerName %s
DocumentRoot /var/www/html

SSLEngine on
SSLCertificateFile /etc/letsencrypt/live/%s/fullchain.pem
SSLCertificateKeyFile /etc/letsencrypt/live/%s/privkey.pem

<Directory /var/www/html>
Header set Access-Control-Allow-Origin "*"
Header set Access-Control-Allow-Methods "GET, POST, OPTIONS"
Header set Access-Control-Allow-Headers "Origin, X-Requested-With, Content-Type, Accept"
AllowOverride All
Require all granted
</Directory>
</VirtualHost>`, fqdn, fqdn, fqdn, fqdn, fqdn)
		} else {
			vhostContent = fmt.Sprintf(`<VirtualHost *:80>
ServerName %s
DocumentRoot /var/www/html

<Directory /var/www/html>
Header set Access-Control-Allow-Origin "*"
Header set Access-Control-Allow-Methods "GET, POST, OPTIONS"
Header set Access-Control-Allow-Headers "Origin, X-Requested-With, Content-Type, Accept"
AllowOverride All
Require all granted
</Directory>
</VirtualHost>`, fqdn)
		}

		os.WriteFile("/etc/apache2/sites-available/000-default.conf", []byte(vhostContent), 0644)
		exec.Command("systemctl", "restart", "apache2").Run()
	} else if webServer == "nginx" {
		fmt.Println("Configuring default Nginx vhost...")
		var vhostContent string
		if useSSL {
			vhostContent = fmt.Sprintf(`server {
    listen 80 default_server;
    server_name %s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl default_server;
    server_name %s;
    root /var/www/html;

    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;

    location / {
        add_header Access-Control-Allow-Origin *;
        add_header Access-Control-Allow-Methods "GET, POST, OPTIONS";
        try_files $uri $uri/ =404;
    }
}`, fqdn, fqdn, fqdn, fqdn)
		} else {
			vhostContent = fmt.Sprintf(`server {
    listen 80 default_server;
    server_name %s;
    root /var/www/html;

    location / {
        add_header Access-Control-Allow-Origin *;
        add_header Access-Control-Allow-Methods "GET, POST, OPTIONS";
        try_files $uri $uri/ =404;
    }
}`, fqdn)
		}
		os.WriteFile("/etc/nginx/sites-available/default", []byte(vhostContent), 0644)
		exec.Command("systemctl", "restart", "nginx").Run()
	}
}
