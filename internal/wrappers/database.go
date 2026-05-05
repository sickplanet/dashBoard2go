package wrappers

// DatabaseConfig holds standard auth config for connection
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

// DatabaseServer is the strategy interface for database engines.
// This allows the core application to manage MariaDB, MySQL, or Postgres seamlessly.
type DatabaseServer interface {
	// Connect establishes the connection to the database root/admin user
	Connect(config DatabaseConfig) error

	// Close terminates the active connection
	Close() error

	// CreateDatabase creates a new empty schema/database
	CreateDatabase(dbName string) error

	// DropDatabase removes a schema/database and all its contents
	DropDatabase(dbName string) error

	// CreateUser creates a new database user with a secure password
	CreateUser(username, password string) error

	// DropUser removes a user completely
	DropUser(username string) error

	// GrantPrivileges grants all privileges on a specific database to a specific user
	GrantPrivileges(dbName, username string) error
}
