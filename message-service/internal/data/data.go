package data

import (
	"database/sql"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/conf"

	_ "github.com/lib/pq"
)

// NewDB creates a database connection
func NewDB(c *conf.Data) (*sql.DB, error) {
	db, err := sql.Open(c.Database.Driver, c.Database.Source)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
