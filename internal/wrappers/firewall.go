package wrappers

import "context"

// FirewallRule represents a tracked firewall rule in the database
type FirewallRule struct {
	ID       int    `json:"id"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"` // ALLOW, DENY, REJECT
	Source   string `json:"source"`
	Comment  string `json:"comment"`
}

// Firewall defines the interface for managing system firewalls
type Firewall interface {
	// Initialize ensures the firewall is installed and active
	Initialize(ctx context.Context) error

	// AddRule adds a new rule and returns the DB tracked ID
	AddRule(ctx context.Context, rule *FirewallRule) error

	// RemoveRule removes a rule from both UFW and DB
	RemoveRule(ctx context.Context, ruleID int) error

	// GetRules returns all rules currently active in the firewall
	GetSystemRules(ctx context.Context) ([]FirewallRule, error)

	// GetDBRules returns all rules tracked in the SQL database
	GetDBRules(ctx context.Context) ([]FirewallRule, error)

	// Sync checks if the DB rules match the System rules, pushing DB rules to the system if mismatch occurs
	Sync(ctx context.Context) (bool, error)
}
