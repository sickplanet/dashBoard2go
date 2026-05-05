package wrappers

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
)

type UFWWrapper struct {
	DB *sql.DB
}

func NewUFWWrapper(db *sql.DB) *UFWWrapper {
	return &UFWWrapper{DB: db}
}

func (u *UFWWrapper) Initialize(ctx context.Context) error { // Enable UFW
	cmd := exec.CommandContext(ctx, "ufw", "--force", "enable")
	if cmd == nil {
		return fmt.Errorf("failed to enable UFW")
	}
	return cmd.Run()
}

func (u *UFWWrapper) AddRule(ctx context.Context, rule *FirewallRule) error {
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
	if u.DB != nil {
		u.DB.Exec("INSERT INTO firewall_rules (port, protocol, action, source, comment) VALUES (?, ?, ?, ?, ?)", rule.Port, rule.Protocol, rule.Action, rule.Source, rule.Comment)
	}
	return nil
}

func (u *UFWWrapper) RemoveRule(ctx context.Context, ruleID int) error {
	if u.DB != nil {
		u.DB.Exec("DELETE FROM firewall_rules WHERE id = ?", ruleID)
	}
	// TODO: Perform the actual underlying ufw delete logic based on the specific rule pulled before deletion
	return nil
}

func (u *UFWWrapper) GetDBRules(ctx context.Context) ([]FirewallRule, error) {
	if u.DB == nil {
		return []FirewallRule{}, nil
	}
	rows, err := u.DB.QueryContext(ctx, "SELECT id, port, protocol, action, source, comment FROM firewall_rules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []FirewallRule
	for rows.Next() {
		var r FirewallRule
		var c sql.NullString
		if err := rows.Scan(&r.ID, &r.Port, &r.Protocol, &r.Action, &r.Source, &c); err == nil {
			r.Comment = c.String
			rules = append(rules, r)
		}
	}
	return rules, nil
}

func (u *UFWWrapper) GetSystemRules(ctx context.Context) ([]FirewallRule, error) {
	cmd := exec.CommandContext(ctx, "ufw", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

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
	// Sync disabled temporarily for modular fixes
	return false, nil
}

