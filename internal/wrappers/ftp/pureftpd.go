package ftp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// FTPUser represents a virtual FTP user account
type FTPUser struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	HomeDir  string `json:"home_dir"`
	UID      string `json:"uid"`      // System user ID to map to
	GID      string `json:"gid"`      // System group ID to map to
	QuotaMB  int    `json:"quota_mb"` // 0 for unlimited
}

// FTPManager defines the interface for managing FTP services and users
type FTPManager interface {
	AddUser(ctx context.Context, user *FTPUser) error
	DeleteUser(ctx context.Context, username string) error
	SetPassword(ctx context.Context, username, password string) error
	UpdateQuota(ctx context.Context, username string, quotaMB int) error
	CommitDB(ctx context.Context) error
}

// PureFTPdWrapper implements FTPManager for Pure-FTPd using `pure-pw`
type PureFTPdWrapper struct {
	DefaultUID string
	DefaultGID string
}

func NewPureFTPdWrapper(uid, gid string) *PureFTPdWrapper {
	return &PureFTPdWrapper{
		DefaultUID: uid,
		DefaultGID: gid,
	}
}

func (p *PureFTPdWrapper) AddUser(ctx context.Context, user *FTPUser) error {
	uid := user.UID
	if uid == "" {
		uid = p.DefaultUID
	}
	gid := user.GID
	if gid == "" {
		gid = p.DefaultGID
	}

	// command: pure-pw useradd <username> -u <uid> -g <gid> -d <homedir>
	cmdArgs := []string{"useradd", user.Username, "-u", uid, "-g", gid, "-d", user.HomeDir}

	if user.QuotaMB > 0 {
		// Set quota (e.g. -q 1000 for 1000 MB)
		cmdArgs = append(cmdArgs, "-q", fmt.Sprintf("%d", user.QuotaMB))
	}

	cmd := exec.CommandContext(ctx, "pure-pw", cmdArgs...)

	// pure-pw prompts twice for the password. We pipe it via stdin.
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n", user.Password, user.Password))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pure-pw useradd failed: %v", err)
	}

	return p.CommitDB(ctx)
}

func (p *PureFTPdWrapper) DeleteUser(ctx context.Context, username string) error {
	// command: pure-pw userdel <username>
	cmd := exec.CommandContext(ctx, "pure-pw", "userdel", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pure-pw userdel failed: %v", err)
	}
	return p.CommitDB(ctx)
}

func (p *PureFTPdWrapper) SetPassword(ctx context.Context, username, password string) error {
	// command: pure-pw passwd <username>
	cmd := exec.CommandContext(ctx, "pure-pw", "passwd", username)
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s\n%s\n", password, password))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pure-pw passwd failed: %v", err)
	}
	return p.CommitDB(ctx)
}

func (p *PureFTPdWrapper) UpdateQuota(ctx context.Context, username string, quotaMB int) error {
	// command: pure-pw usermod <username> -q <quota>
	cmdArgs := []string{"usermod", username}
	if quotaMB > 0 {
		cmdArgs = append(cmdArgs, "-q", fmt.Sprintf("%d", quotaMB))
	} else {
		// Quota of 0 usually means unlimited, but pure-pw syntax expects empty for unlimited.
		// Leaving it simple for now or you can pass "" to clear
		cmdArgs = append(cmdArgs, "-q", "''")
	}

	cmd := exec.CommandContext(ctx, "pure-pw", cmdArgs...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pure-pw usermod failed: %v", err)
	}
	return p.CommitDB(ctx)
}

func (p *PureFTPdWrapper) CommitDB(ctx context.Context) error {
	// command: pure-pw mkdb
	// This compiles the /etc/pure-ftpd/pureftpd.passwd file into the pureftpd.pdb binary format
	cmd := exec.CommandContext(ctx, "pure-pw", "mkdb")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pure-pw mkdb failed: %v", err)
	}
	return nil
}
