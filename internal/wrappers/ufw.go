package wrappers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// UFWWrapper implements Firewall interface via ufw commands
type UFWWrapper struct {
	// We would inject a *sql.DB here in production
	// DB *sql.DB
}

func NewUFWWrapper() *UFWWrapper {
	return &UFWWrapper{}
}

// In a real implementation this interacts with the sqlite or PG db.
// We use a mock slice for now to represent our tracked SQL rules.
var mockSQLFirewallDB = []FirewallRule{
	{ID: 1, Port: "22", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "SSH"},
	{ID: 2, Port: "80", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "HTTP"},
	{ID: 3, Port: "443", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "HTTPS"},
	{ID: 4, Port: "21", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "FTP"},
	{ID: 5, Port: "25", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "SMTP"},
	{ID: 6, Port: "110", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "POP3"},
	{ID: 7, Port: "143", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "IMAP"},
	{ID: 8, Port: "465", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "SMTPS"},
	{ID: 9, Port: "587", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "SMTP Submission"},
	{ID: 10, Port: "993", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "IMAPS"},
	{ID: 11, Port: "995", Protocol: "tcp", Action: "ALLOW", Source: "Anywhere", Comment: "POP3S"}}

func (u *UFWWrapper) Initialize(ctx context.Context) error { // Enable UFW
	cmd := exec.CommandContext(ctx, "ufw", "--force", "enable")
	if cmd == nil {
		return fmt.Errorf("failed to enable UFW")
	}
	return cmd.Run()
}

func (u *UFWWrapper) AddRule(ctx context.Context, rule *FirewallRule) error {
	// e.g. ufw allow 80/tcp
	cmdArgs := []string{strings.ToLower(rule.Action)}
	if rule.Source != "" && rule.Source != "Anywhere" {
		cmdArgs = append(cmdArgs, "from", rule.Source)
	}
	cmdArgs = append(cmdArgs, "to", "any", "port", rule.Port)
	if rule.Protocol != "" {
		cmdArgs = append(cmdArgs, "proto", rule.Protocol)
	}

	cmd := exec.CommandContext(ctx, "ufw", cmdArgs...)
	err := cmd.Run()
	if err != nil {
		return err
	}
	// Simulate adding to SQL DB
	rule.ID = len(mockSQLFirewallDB) + 1
	mockSQLFirewallDB = append(mockSQLFirewallDB, *rule)
	return nil
}

func (u *UFWWrapper) RemoveRule(ctx context.Context, ruleID int) error {
	// Remove from DB first
	for i, r := range mockSQLFirewallDB {
		if r.ID == ruleID {
			// Find ufw rule and delete it. Here we construct a matching rule and run `ufw delete allow 80/tcp`
			mockSQLFirewallDB = append(mockSQLFirewallDB[:i], mockSQLFirewallDB[i+1:]...)
			cmdArgs := []string{"delete", strings.ToLower(r.Action), r.Port + "/" + r.Protocol}
			cmd := exec.CommandContext(ctx, "ufw", cmdArgs...)
			return cmd.Run()
		}
	}
	return fmt.Errorf("rule not found in DB")
}

func (u *UFWWrapper) GetDBRules(ctx context.Context) ([]FirewallRule, error) {
	// Select * from firewall_rules;
	return mockSQLFirewallDB, nil
}

func (u *UFWWrapper) GetSystemRules(ctx context.Context) ([]FirewallRule, error) {
	// ufw status numbered
	cmd := exec.CommandContext(ctx, "ufw", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	// Very simplified parser for 'ufw status' output
	// Expected lines: "80/tcp                     ALLOW       Anywhere"
	var rules []FirewallRule
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ALLOW") || strings.Contains(line, "DENY") || strings.Contains(line, "REJECT") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				portProto := strings.Split(parts[0], "/")
				port := portProto[0]
				proto := ""
				if len(portProto) > 1 {
					proto = portProto[1]
				}
				rules = append(rules, FirewallRule{
					Port:     port,
					Protocol: proto,
					Action:   parts[1],
					Source:   parts[2],
				})
			}
		}
	}
	return rules, nil
}

func (u *UFWWrapper) Sync(ctx context.Context) (bool, error) {
	dbRules, err := u.GetDBRules(ctx)
	if err != nil {
		return false, err
	}
	sysRules, err := u.GetSystemRules(ctx)
	if err != nil {
		return false, err
	}

	// Basic check: compare lengths and simple match. UFW numbering changes on delete,
	// so tracking against a stable DB set is crucial.
	mismatch := len(dbRules) != len(sysRules)

	if mismatch {
		// Flush ufw and recreate from DB (auto-reconfigure)
		// We'd send an alert here normally
		exec.CommandContext(ctx, "ufw", "--force", "reset").Run()
		exec.CommandContext(ctx, "ufw", "--force", "enable").Run()

		for _, r := range dbRules {
			// Ignore mock DB update since we already pull from dbRules
			cmdArgs := []string{strings.ToLower(r.Action), r.Port + "/" + r.Protocol}
			exec.CommandContext(ctx, "ufw", cmdArgs...).Run()
		}
		return true, nil // Returns true if sync performed reconfiguration
	}
	return false, nil
}
