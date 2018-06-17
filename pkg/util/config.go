package util

import (
	"database/sql"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	_ "github.com/mattn/go-sqlite3" // register sqlite3 sql driver
)

// Config holds various settings.
type Config struct {
	DBFile      string
	Org         string
	Repo        string
	AccessToken string

	db  *sql.DB
	tty bool
}

// Populate sets up the internal state for this Config object.
// If this method returns successfully, the caller promises to
// call the Close method once not using the Config any more.
func (c *Config) Populate() error {
	db, err := sql.Open("sqlite3", c.DBFile)
	if err != nil {
		return err
	}

	c.db = db
	c.tty = terminal.IsTerminal(int(os.Stdout.Fd()))
	return nil
}

// DB returns the database connection. Populate must have been
// called.
func (c *Config) DB() *sql.DB {
	return c.db
}

// IsTTY returns whether the output is known to be a terminal.
func (c *Config) IsTTY() bool {
	return c.tty
}

// Close releases resources held by this Config.
func (c *Config) Close() {
	_ = c.db.Close()
}
