package wrappers

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
)

// PostfixDovecotWrapper implements the MailServer interface.
// Standard setups use a Database (MariaDB/Postgres) to store virtual domains, users, and aliases.
// Postfix and Dovecot are configured to query this DB. Therefore, managing email is largely a DB task
// coupled with triggering service reloads dynamically via the wrapper.
type PostfixDovecotWrapper struct {
	db *sql.DB // Connection to the control panel's virtual mail DB
}

// NewPostfixDovecotWrapper links the DB backend for virtual user mapping
func NewPostfixDovecotWrapper(db *sql.DB) *PostfixDovecotWrapper {
	return &PostfixDovecotWrapper{db: db}
}

// Generates a Dovecot compatible SHA512-CRYPT hashed password natively
func (m *PostfixDovecotWrapper) generateMailPassword(plainPass string) (string, error) {
	// Dovecot ships with 'doveadm pw', a foolproof way to generate scheme-correct hashes.
	cmd := exec.Command("doveadm", "pw", "-s", "SHA512-CRYPT", "-p", plainPass)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("doveadm failed to hash password: %v", err)
	}
	return string(out), nil
}

func (m *PostfixDovecotWrapper) CreateMailDomain(domain string) error {
	// Virtual mapping: Insert domain into `virtual_domains` SQL table
	_, err := m.db.Exec("INSERT INTO virtual_domains (name) VALUES (?)", domain)
	return err
}

func (m *PostfixDovecotWrapper) DeleteMailDomain(domain string) error {
	_, err := m.db.Exec("DELETE FROM virtual_domains WHERE name = ?", domain)
	return err
}

func (m *PostfixDovecotWrapper) CreateMailbox(config MailboxConfig) error {
	hash, err := m.generateMailPassword(config.Password)
	if err != nil {
		return err
	}

	query := `INSERT INTO virtual_users (domain_id, email, password, quota_mb, antispam) 
	          VALUES ((SELECT id FROM virtual_domains WHERE name=?), ?, ?, ?, ?)`

	_, err = m.db.Exec(query, config.Domain, config.Email, hash, config.QuotaMB, config.Antispam)
	return err
}

func (m *PostfixDovecotWrapper) DeleteMailbox(email string) error {
	_, err := m.db.Exec("DELETE FROM virtual_users WHERE email = ?", email)
	return err
}

func (m *PostfixDovecotWrapper) UpdatePassword(email, newPassword string) error {
	hash, err := m.generateMailPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = m.db.Exec("UPDATE virtual_users SET password = ? WHERE email = ?", hash, email)
	return err
}

func (m *PostfixDovecotWrapper) UpdateQuota(email string, quotaMB int) error {
	// Dovecot dict integration natively handles updating quotas live.
	// Updating the SQL value applies to Dovecot immediately on the next IMAP session login.
	_, err := m.db.Exec("UPDATE virtual_users SET quota_mb = ? WHERE email = ?", quotaMB, email)
	return err
}

func (m *PostfixDovecotWrapper) CreateAlias(alias MailAlias) error {
	query := `INSERT INTO virtual_aliases (domain_id, source, destination) 
	          VALUES ((SELECT id FROM virtual_domains WHERE name=?), ?, ?)`
	_, err := m.db.Exec(query, alias.Domain, alias.Source, alias.Destination)
	return err
}

func (m *PostfixDovecotWrapper) DeleteAlias(source string) error {
	_, err := m.db.Exec("DELETE FROM virtual_aliases WHERE source = ?", source)
	return err
}

// SetAntispamPolicy controls Amavisd/SpamAssassin integration per user
func (m *PostfixDovecotWrapper) SetAntispamPolicy(email string, blockSpam bool) error {
	// Example integration: Amavis queries the identical database to check if a user
	// bypasses spam checks, or updates global thresholds.
	_, err := m.db.Exec("UPDATE virtual_users SET antispam = ? WHERE email = ?", blockSpam, email)
	return err
}

// Reload applies system-wide config file updates to Postfix, Dovecot, and Amavis
func (m *PostfixDovecotWrapper) Reload() error {
	// Normally Virtual Mail databases update on-the-fly and need no reload.
	// But if modifying postfix main.cf / master.cf / dovecot.conf:
	exec.Command("systemctl", "reload", "postfix").Run()
	exec.Command("systemctl", "reload", "dovecot").Run()
	exec.Command("systemctl", "reload", "amavis").Run() // or amavisd-new
	return nil
}

// Add InitDatabase for Mail structure
func (m *PostfixDovecotWrapper) InitDatabase() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS virtual_domains (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT UNIQUE NOT NULL
);`,
		`CREATE TABLE IF NOT EXISTS virtual_users (
id INTEGER PRIMARY KEY AUTOINCREMENT,
domain_id INTEGER,
email TEXT UNIQUE NOT NULL,
password TEXT NOT NULL,
quota_mb INTEGER DEFAULT 0,
antispam BOOLEAN DEFAULT 1,
FOREIGN KEY(domain_id) REFERENCES virtual_domains(id) ON DELETE CASCADE
);`,
		`CREATE TABLE IF NOT EXISTS virtual_aliases (
id INTEGER PRIMARY KEY AUTOINCREMENT,
domain_id INTEGER,
source TEXT NOT NULL,
destination TEXT NOT NULL,
FOREIGN KEY(domain_id) REFERENCES virtual_domains(id) ON DELETE CASCADE
);`,
	}
	for _, q := range queries {
		if _, err := m.db.Exec(q); err != nil {
			return fmt.Errorf("failed to init mail table: %v", err)
		}
	}
	return nil
}

func (m *PostfixDovecotWrapper) CreateCatchAll(domain, destination string) error {
	// A catch-all in postfix expects source to be `@domain.com`
	source := "@" + domain
	query := `INSERT INTO virtual_aliases (domain_id, source, destination) 
  VALUES ((SELECT id FROM virtual_domains WHERE name=?), ?, ?)`
	_, err := m.db.Exec(query, domain, source, destination)
	return err
}

func (m *PostfixDovecotWrapper) DeleteCatchAll(domain string) error {
	source := "@" + domain
	_, err := m.db.Exec("DELETE FROM virtual_aliases WHERE source = ?", source)
	return err
}

// InstallConfigs rewrites Postfix and Dovecot configurations to bind to the panel's SQLite database
func (m *PostfixDovecotWrapper) InstallConfigs(sqliteDBPath string) error {
	// 1. Write Postfix SQLite map configurations
	domainMap := fmt.Sprintf(`dbpath = %s
query = SELECT name FROM virtual_domains WHERE name='%s'
`, sqliteDBPath, "%s")

	mailboxMap := fmt.Sprintf(`dbpath = %s
query = SELECT 1 FROM virtual_users WHERE email='%s'
`, sqliteDBPath, "%s")

	aliasMap := fmt.Sprintf(`dbpath = %s
query = SELECT destination FROM virtual_aliases WHERE source='%s'
`, sqliteDBPath, "%s")

	os.WriteFile("/etc/postfix/sqlite_virtual_domains.cf", []byte(domainMap), 0644)
	os.WriteFile("/etc/postfix/sqlite_virtual_mailboxes.cf", []byte(mailboxMap), 0644)
	os.WriteFile("/etc/postfix/sqlite_virtual_aliases.cf", []byte(aliasMap), 0644)

	// 2. Reconfigure Postfix main.cf to use these maps (Appended or sed'd normally)
	// We run postconf to inject the values safely
	exec.Command("postconf", "-e", "virtual_mailbox_domains = sqlite:/etc/postfix/sqlite_virtual_domains.cf").Run()
	exec.Command("postconf", "-e", "virtual_alias_maps = sqlite:/etc/postfix/sqlite_virtual_aliases.cf").Run()
	exec.Command("postconf", "-e", "virtual_mailbox_maps = sqlite:/etc/postfix/sqlite_virtual_mailboxes.cf").Run()

	// 3. Write Dovecot SQL auth config mapping passwords and quotas
	dovecotSql := fmt.Sprintf(`driver = sqlite
connect = %s
default_pass_scheme = SHA512-CRYPT
password_query = SELECT email as user, password FROM virtual_users WHERE email = '%s'
user_query = SELECT email as user, 'vmail' as uid, 'vmail' as gid, '/var/vmail/' || email as home, '*:bytes=' || quota_mb as quota_rule FROM virtual_users WHERE email = '%s'
`, sqliteDBPath, "%u", "%u")

	os.WriteFile("/etc/dovecot/dovecot-sql.conf.ext", []byte(dovecotSql), 0644)

	m.Reload()
	return nil
}
