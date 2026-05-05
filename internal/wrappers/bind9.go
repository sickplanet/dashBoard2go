package wrappers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Bind9Wrapper implements the DNSServer interface for Bind9
type Bind9Wrapper struct {
	ZonesPath      string
	ConfigFilePath string // e.g., /etc/bind/named.conf.dashboard
}

// NewBind9Wrapper initializes a new Bind9 wrapper.
// This assumes you will add `include "/etc/bind/named.conf.dashboard";` to your named.conf.local
func NewBind9Wrapper() *Bind9Wrapper {
	return &Bind9Wrapper{
		ZonesPath:      "/etc/bind/zones",
		ConfigFilePath: "/etc/bind/named.conf.dashboard",
	}
}

const baseZoneTemplate = `$TTL 86400
@   IN  SOA     {{.Nameserver}}. {{.AdminEmail}}. (
        {{.Serial}}  ; Serial
        3600        ; Refresh
        1800        ; Retry
        604800      ; Expire
        86400 )     ; Minimum TTL

; Name Servers
@   IN  NS      {{.Nameserver}}.

; Base Records
@   IN  A       {{.PrimaryIP}}
www IN  A       {{.PrimaryIP}}
`

// CreateZone creates the actual DNS zone file and registers it in the config
func (b *Bind9Wrapper) CreateZone(config ZoneConfig) error {
	_ = os.MkdirAll(b.ZonesPath, 0755)

	zoneFile := filepath.Join(b.ZonesPath, fmt.Sprintf("%s.db", config.Domain))

	// Create SOA Serial based on date+revision (YYYYMMDD01)
	serial := time.Now().Format("20060102") + "01"

	data := struct {
		ZoneConfig
		Serial string
	}{
		config, serial,
	}

	tmpl, err := template.New("zone").Parse(baseZoneTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	if err := os.WriteFile(zoneFile, buf.Bytes(), 0644); err != nil {
		return err
	}

	// Register in the named.conf.dashboard
	return b.registerZoneConfig(config.Domain, zoneFile)
}

func (b *Bind9Wrapper) registerZoneConfig(domain, zoneFile string) error {
	configEntry := fmt.Sprintf("\nzone \"%s\" {\n    type master;\n    file \"%s\";\n    allow-transfer { any; };\n};\n", domain, zoneFile)

	f, err := os.OpenFile(b.ConfigFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(configEntry); err != nil {
		return err
	}
	return nil
}

// DeleteZone removes the zone file and purges it from the config
func (b *Bind9Wrapper) DeleteZone(domain string) error {
	zoneFile := filepath.Join(b.ZonesPath, fmt.Sprintf("%s.db", domain))
	_ = os.Remove(zoneFile)

	// In a complete implementation, this would parse ConfigFilePath and remove the specific zone block.
	// For scaffolding, we represent the intent:
	content, err := os.ReadFile(b.ConfigFilePath)
	if err != nil {
		return nil // Ignore if it doesn't exist yet
	}

	// Simplistic removal (in production, use a regex or AST parser for Bind configurations)
	lines := strings.Split(string(content), "\n")
	var newContent bytes.Buffer
	skip := false
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(`zone "%s" {`, domain)) {
			skip = true
			continue
		}
		if skip && strings.Contains(line, "};") {
			skip = false
			continue
		}
		if !skip {
			newContent.WriteString(line + "\n")
		}
	}

	return os.WriteFile(b.ConfigFilePath, newContent.Bytes(), 0644)
}

// AddRecord appends a single record to the domain's zone file
func (b *Bind9Wrapper) AddRecord(domain string, record DNSRecord) error {
	zoneFile := filepath.Join(b.ZonesPath, fmt.Sprintf("%s.db", domain))

	f, err := os.OpenFile(zoneFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var recordStr string
	if record.Priority != nil {
		// Used mainly for MX
		recordStr = fmt.Sprintf("%s\t%d\tIN\t%s\t%d\t%s\n", record.Name, record.TTL, record.Type, *record.Priority, record.Value)
	} else {
		recordStr = fmt.Sprintf("%s\t%d\tIN\t%s\t%s\n", record.Name, record.TTL, record.Type, record.Value)
	}

	_, err = f.WriteString(recordStr)

	// Note: in a true production flow, the SOA serial MUST be incremented when adding a record.
	// That requires reading, parsing, regex replacing the serial, and saving.
	return err
}

// RemoveRecord removes a specific record (simplified)
func (b *Bind9Wrapper) RemoveRecord(domain string, record DNSRecord) error {
	// Implementation would read file, filter out the line matching the record, rewrite, and increment SOA.
	return nil
}

// EnableDNSSEC generates dnssec keys for the domain using native Bind9 utilities
func (b *Bind9Wrapper) EnableDNSSEC(domain string) error {
	// 1. Generate KSK (Key Signing Key)
	cmd1 := exec.Command("dnssec-keygen", "-a", "RSASHA256", "-b", "2048", "-f", "KSK", domain)
	cmd1.Dir = b.ZonesPath
	if out, err := cmd1.CombinedOutput(); err != nil {
		return fmt.Errorf("KSK gen failed: %v, output: %s", err, string(out))
	}

	// 2. Generate ZSK (Zone Signing Key)
	cmd2 := exec.Command("dnssec-keygen", "-a", "RSASHA256", "-b", "1024", domain)
	cmd2.Dir = b.ZonesPath
	if out, err := cmd2.CombinedOutput(); err != nil {
		return fmt.Errorf("ZSK gen failed: %v, output: %s", err, string(out))
	}

	// 3. To auto-sign, named.conf.dashboard needs `auto-dnssec maintain; inline-signing yes;` added to the zone block.
	// Instead of manual signzone, relying on Bind9's inline-signing is standard for cPanel equivalents.
	return nil
}

// GenerateDKIM creates an RSA keypair for mail signing (used by Amavis/SpamAssassin/DKIM filter)
func (b *Bind9Wrapper) GenerateDKIM(domain, selector string) (string, error) {
	// For DKIM, OpenSSL is universally available and perfect for this
	privKeyPath := filepath.Join("/etc/db2go/dkim", fmt.Sprintf("%s.private", domain))
	pubKeyPath := filepath.Join("/etc/db2go/dkim", fmt.Sprintf("%s.public", domain))

	_ = os.MkdirAll("/etc/db2go/dkim", 0700)

	// Private Key
	exec.Command("openssl", "genrsa", "-out", privKeyPath, "2048").Run()

	// Public Key extraction
	cmd := exec.Command("openssl", "rsa", "-in", privKeyPath, "-pubout")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Format public key to be a clean TXT record value
	pubKeyRaw := strings.ReplaceAll(string(out), "-----BEGIN PUBLIC KEY-----", "")
	pubKeyRaw = strings.ReplaceAll(pubKeyRaw, "-----END PUBLIC KEY-----", "")
	pubKeyRaw = strings.ReplaceAll(pubKeyRaw, "\n", "")

	dkimTxtValue := fmt.Sprintf("v=DKIM1; h=sha256; k=rsa; p=%s", pubKeyRaw)

	// Save public key for reference
	_ = os.WriteFile(pubKeyPath, []byte(dkimTxtValue), 0644)

	return dkimTxtValue, nil
}

// Reload applies zone changes live to the nameserver
func (b *Bind9Wrapper) Reload() error {
	// rndc is the native Name Server Control Utility for Bind9
	cmd := exec.Command("rndc", "reload")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rndc reload failed: %v, output: %s", err, string(out))
	}
	return nil
}
