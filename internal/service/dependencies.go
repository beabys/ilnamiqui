package service

import "database/sql"

// Config defines what the service needs from project configuration.
type Config interface {
	DBPath() (string, error)
	ProjectSlug() (string, error)
	FindProjectRoot() (string, error)
}

// DBOpener defines database creation and migration.
type DBOpener interface {
	NewDB(path string) (DB, error)
	RunMigrations(sqlDB *sql.DB) error
}

// DB defines the minimal subset of db.DB that the service uses.
type DB interface {
	SQLDB() *sql.DB
	Close() error
}
