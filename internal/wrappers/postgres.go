package wrappers

import (
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/lib/pq"
)

// PostgresWrapper implements the DatabaseServer interface for PostgreSQL
type PostgresWrapper struct {
	db *sql.DB
}

// NewPostgresWrapper creates a new wrapper instance
func NewPostgresWrapper() *PostgresWrapper {
	return &PostgresWrapper{}
}

// Connect opens a connection to the PostgreSQL server.
// Connects to the default "postgres" database initially to perform administrative tasks.
func (p *PostgresWrapper) Connect(config DatabaseConfig) error {
	port := config.Port
	if port == 0 {
		port = 5432
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		config.Host, port, config.User, config.Password)

	// If local and empty password, attempt peer authentication over unix socket
	if config.Host == "localhost" && config.Password == "" {
		dsn = fmt.Sprintf("user=%s dbname=postgres sslmode=disable", config.User)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open Postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping Postgres: %w", err)
	}

	p.db = db
	return nil
}

// Close closes the database connection
func (p *PostgresWrapper) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// CreateDatabase executes CREATE DATABASE schema
func (p *PostgresWrapper) CreateDatabase(dbName string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}

	// Postgres does not support "IF NOT EXISTS" for CREATE DATABASE natively in the same way.
	// We'll check if it exists first.
	var exists bool
	queryCheck := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = '%s')", dbName)
	err := p.db.QueryRow(queryCheck).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Postgres requires double quotes for identifiers
		_, err = p.db.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
		return err
	}
	return nil
}

// DropDatabase removes a database
func (p *PostgresWrapper) DropDatabase(dbName string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}
	_, err := p.db.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
	return err
}

// CreateUser creates a role/user with login permissions
func (p *PostgresWrapper) CreateUser(username, password string) error {
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}

	var exists bool
	queryCheck := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = '%s')", username)
	err := p.db.QueryRow(queryCheck).Scan(&exists)
	if err != nil {
		return err
	}

	safePass := regexp.MustCompile(`'`).ReplaceAllString(password, `''`)

	if !exists {
		_, err = p.db.Exec(fmt.Sprintf(`CREATE ROLE "%s" WITH LOGIN PASSWORD '%s'`, username, safePass))
		return err
	} else {
		// If exists, just update the password to ensure it's correct
		_, err = p.db.Exec(fmt.Sprintf(`ALTER ROLE "%s" WITH PASSWORD '%s'`, username, safePass))
		return err
	}
}

// DropUser drops the user/role from Postgres
func (p *PostgresWrapper) DropUser(username string) error {
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}
	_, err := p.db.Exec(fmt.Sprintf(`DROP ROLE IF EXISTS "%s"`, username))
	return err
}

// GrantPrivileges applies all limits of database control to the specific user
func (p *PostgresWrapper) GrantPrivileges(dbName, username string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}

	// Grant all privileges on the database
	query := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, dbName, username)
	if _, err := p.db.Exec(query); err != nil {
		return err
	}

	return nil
}
