package postgres

import "database/sql"

// Pinger реализует проверку соединения с БД.
type Pinger struct {
	db *sql.DB
}

func NewPinger(db *sql.DB) *Pinger {
	return &Pinger{db: db}
}

func (p *Pinger) Ping() error {
	return p.db.Ping()
}
