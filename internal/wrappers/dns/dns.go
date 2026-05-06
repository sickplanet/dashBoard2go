package dns

// ZoneConfig holds the base configuration for a new DNS zone
type ZoneConfig struct {
	Domain     string
	AdminEmail string // e.g., admin.domain.com (dot instead of @)
	PrimaryIP  string // Base A record IP
	Nameserver string // e.g., ns1.domain.com
}

// DNSRecord represents a single DNS entry (A, AAAA, CNAME, TXT, MX, etc.)
type DNSRecord struct {
	Type     string
	Name     string // e.g., "@", "www", "mail"
	Value    string // e.g., "192.168.1.1", "v=DKIM1; k=rsa; p=..."
	TTL      int    // e.g., 3600
	Priority *int   // Used primarily for MX records
}

// DNSServer is the strategy interface for DNS engines like Bind9
type DNSServer interface {
	// CreateZone generates the base zone file and registers it
	CreateZone(config ZoneConfig) error

	// DeleteZone completely removes a domain's zone and its registration
	DeleteZone(domain string) error

	// AddRecord appends a single DNS record to the domain's zone
	AddRecord(domain string, record DNSRecord) error

	// RemoveRecord removes a matching DNS record from the domain's zone
	RemoveRecord(domain string, record DNSRecord) error

	// EnableDNSSEC generates keys and signs the zone (or configures auto-dnssec)
	EnableDNSSEC(domain string) error

	// GenerateDKIM generates a DKIM private/public keypair for mail signing
	GenerateDKIM(domain, selector string) (string, error)

	// Reload signals the DNS server to load new zone configurations gracefully
	Reload() error
}
