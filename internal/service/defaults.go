package service

import (
	"database/sql"

	"github.com/beabys/ilnamiqui/internal/config"
	"github.com/beabys/ilnamiqui/internal/db"
)

// defaultConfig wraps the config package functions.
type defaultConfig struct{}

func (defaultConfig) DBPath() (string, error)           { return config.DBPath() }
func (defaultConfig) ProjectSlug() (string, error)       { return config.ProjectSlug() }
func (defaultConfig) FindProjectRoot() (string, error)   { return config.FindProjectRoot() }

// defaultDBOpener wraps the db package functions.
type defaultDBOpener struct{}

func (defaultDBOpener) NewDB(path string) (DB, error) {
	d, err := db.NewDB(path)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (defaultDBOpener) RunMigrations(sqlDB *sql.DB) error { return db.RunMigrations(sqlDB) }

// DefaultConfig returns a Config backed by the real config package.
func DefaultConfig() Config { return defaultConfig{} }

// DefaultDBOpener returns a DBOpener backed by the real db package.
func DefaultDBOpener() DBOpener { return defaultDBOpener{} }
