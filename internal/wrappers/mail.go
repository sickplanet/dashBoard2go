package wrappers

// MailboxConfig defines settings for a specific email account
type MailboxConfig struct {
	Domain   string
	Email    string // e.g., user@domain.com
	Password string // Hashed/Crypt string dependent on Dovecot config
	QuotaMB  int    // Quota limits in Megabytes (0 = unlimited)
	Antispam bool   // Whether Amavis/SpamAssassin should actively filter it
}

// MailAlias defines email forwards
type MailAlias struct {
	Domain      string
	Source      string // alias@domain.com
	Destination string // real_user@domain.com or external@gmail.com
}

// MailServer is the strategy interface managing the full mail abstraction layer.
// This typically controls Postfix (MTA), Dovecot (IMAP/POP3), and Amavis/SpamAssassin.
type MailServer interface {
	// Domains
	CreateMailDomain(domain string) error
	DeleteMailDomain(domain string) error

	// Mailboxes
	CreateMailbox(config MailboxConfig) error
	DeleteMailbox(email string) error
	UpdatePassword(email, newPassword string) error
	UpdateQuota(email string, quotaMB int) error

	// Aliases & Forwards
	CreateAlias(alias MailAlias) error
	DeleteAlias(source string) error
	CreateCatchAll(domain, destination string) error
	DeleteCatchAll(domain string) error

	// Database Initialization
	InstallConfigs(sqliteDBPath string) error
	InitDatabase() error

	// Spam Filter Toggle (Amavis / SpamAssassin integration)
	SetAntispamPolicy(email string, blockSpam bool) error

	// Reload applies configuration changes
	Reload() error
}
