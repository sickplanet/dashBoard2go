package database

import (
	"database/sql"
	"fmt"
	"regexp"

	_ "github.com/go-sql-driver/mysql"
)

// MariaDBWrapper implements the DatabaseServer interface for MariaDB/MySQL
type MariaDBWrapper struct {
	db *sql.DB
}

// NewMariaDBWrapper creates a new wrapper instance
func NewMariaDBWrapper() *MariaDBWrapper {
	return &MariaDBWrapper{}
}

// sanitizeIdentifier prevents SQL injection since DDL commands (CREATE USER/DATABASE)
// do not support standard prepared statement parameterization.
func sanitizeIdentifier(identifier string) error {
	// Only allow alphanumeric and underscores for DB/User names
	matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", identifier)
	if err != nil || !matched {
		return fmt.Errorf("invalid identifier format: %s", identifier)
	}
	return nil
}

// Connect opens a connection to the MariaDB server
func (m *MariaDBWrapper) Connect(config DatabaseConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?parseTime=true",
		config.User, config.Password, config.Host, config.Port)

	// If running local testing without pass on Debian socket, use unix socket mapping instead:
	if config.Host == "localhost" && config.Password == "" {
		dsn = fmt.Sprintf("%s@unix(/var/run/mysqld/mysqld.sock)/", config.User)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MariaDB connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MariaDB: %w", err)
	}

	m.db = db
	return nil
}

// Close closes the database connection
func (m *MariaDBWrapper) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// CreateDatabase executes CREATE DATABASE schema
func (m *MariaDBWrapper) CreateDatabase(dbName string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}
	_, err := m.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName))
	return err
}

// DropDatabase completely drops a DB instance
func (m *MariaDBWrapper) DropDatabase(dbName string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}
	_, err := m.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
	return err
}

// CreateUser creates the system user in the system DB
func (m *MariaDBWrapper) CreateUser(username, password string) error {
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}
	// Note: Password can contain special chars, so we use string formatting but escape it.
	// A better way is using native string formatting cautiously. Since DDL doesn't allow prepared params:
	// We'll just carefully format and rely on single quotes (not entirely foolproof against quotes inside password).
	// Real world: we should escape single quotes in the password string.
	safePass := regexp.MustCompile(`'`).ReplaceAllString(password, `''`)
	query := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", username, safePass)
	_, err := m.db.Exec(query)
	return err
}

// DropUser drops the user from MariaDB
func (m *MariaDBWrapper) DropUser(username string) error {
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}
	_, err := m.db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", username))
	return err
}

// GrantPrivileges applies all limits of database control to the specific user
func (m *MariaDBWrapper) GrantPrivileges(dbName, username string) error {
	if err := sanitizeIdentifier(dbName); err != nil {
		return err
	}
	if err := sanitizeIdentifier(username); err != nil {
		return err
	}
	query := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", dbName, username)
	if _, err := m.db.Exec(query); err != nil {
		return err
	}
	_, err := m.db.Exec("FLUSH PRIVILEGES")
	return err
}
