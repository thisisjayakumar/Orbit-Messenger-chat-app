package data

import (
	"database/sql"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/message-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	_ "github.com/lib/pq"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewMessageRepo, NewDB)

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

// Data .
type Data struct {
	// TODO wrapped database client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{}, cleanup, nil
}
