package store

import (
	"context"
	"database/sql"
	"time"
)

// SQLStore persists servers and samples through database/sql. It uses the schema
// in migrations/ and plain "?" placeholders, so bring a driver that speaks them
// (SQLite, MySQL). The driver is registered by the caller via a blank import.
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(driver, dsn string) (*SQLStore, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	return &SQLStore{db: db}, nil
}

func (s *SQLStore) Close() error { return s.db.Close() }

func (s *SQLStore) UpsertServer(ctx context.Context, host string, port int) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM servers WHERE host = ? AND port = ?`, host, port).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := s.db.ExecContext(ctx, `INSERT INTO servers (host, port) VALUES (?, ?)`, host, port)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLStore) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, host, port, online, players, max_players, version, motd, last_check FROM servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Server
	for rows.Next() {
		var srv Server
		var version, motd sql.NullString
		var last sql.NullTime
		if err := rows.Scan(&srv.ID, &srv.Host, &srv.Port, &srv.Online, &srv.Players, &srv.MaxPlayers, &version, &motd, &last); err != nil {
			return nil, err
		}
		srv.Version = version.String
		srv.MOTD = motd.String
		if last.Valid {
			srv.LastCheck = last.Time
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

func (s *SQLStore) UpdateStatus(ctx context.Context, serverID int64, st Status) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE servers SET online = ?, players = ?, max_players = ?, version = ?, motd = ?, last_check = ? WHERE id = ?`,
		st.Online, st.Players, st.MaxPlayers, st.Version, st.MOTD, time.Now(), serverID)
	return err
}

func (s *SQLStore) SaveSample(ctx context.Context, serverID int64, st Status) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO status_samples (server_id, online, players, max_players, latency_ms, sampled_at) VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, st.Online, st.Players, st.MaxPlayers, st.LatencyMS, time.Now())
	return err
}

func (s *SQLStore) RecentSamples(ctx context.Context, serverID int64, since time.Time) ([]Sample, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT server_id, online, players, max_players, latency_ms, sampled_at FROM status_samples WHERE server_id = ? AND sampled_at >= ? ORDER BY sampled_at`,
		serverID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Sample
	for rows.Next() {
		var smp Sample
		if err := rows.Scan(&smp.ServerID, &smp.Online, &smp.Players, &smp.MaxPlayers, &smp.LatencyMS, &smp.At); err != nil {
			return nil, err
		}
		out = append(out, smp)
	}
	return out, rows.Err()
}

func (s *SQLStore) PruneSamples(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM status_samples WHERE sampled_at < ?`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
