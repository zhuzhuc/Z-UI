package storage

import (
	"strings"
	"time"
)

type ClientTraffic struct {
	ID         int64      `json:"id"`
	InboundID  int64      `json:"inboundId"`
	Email      string     `json:"email"`
	Uplink     int64      `json:"uplink"`
	Downlink   int64      `json:"downlink"`
	Total      int64      `json:"total"`
	ExpiryTime *time.Time `json:"expiryTime,omitempty"`
	Enable     bool       `json:"enable"`
}

// MigrateClientTraffic creates the client_traffic table if it doesn't exist.
func (s *Store) MigrateClientTraffic() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS client_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    inbound_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    up INTEGER NOT NULL DEFAULT 0,
    down INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL DEFAULT 0,
    expiry_time TEXT NOT NULL DEFAULT '',
    enable INTEGER NOT NULL DEFAULT 1,
    UNIQUE(inbound_id, email)
);`
	_, err := s.db.Exec(ddl)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) ListClientTrafficByInbound(inboundID int64) ([]ClientTraffic, error) {
	rows, err := s.db.Query(`
SELECT id, inbound_id, email, up, down, total, expiry_time, enable
FROM client_traffic
WHERE inbound_id = ?
ORDER BY id ASC`, inboundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ClientTraffic
	for rows.Next() {
		var ct ClientTraffic
		var expiryRaw string
		var enableInt int
		if err := rows.Scan(&ct.ID, &ct.InboundID, &ct.Email, &ct.Uplink, &ct.Downlink, &ct.Total, &expiryRaw, &enableInt); err != nil {
			return nil, err
		}
		ct.Enable = enableInt == 1
		if t, ok := parseDBTime(expiryRaw); ok {
			ct.ExpiryTime = &t
		}
		items = append(items, ct)
	}
	return items, rows.Err()
}

func (s *Store) UpsertClientTrafficByEmail(email string, up, down int64) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	// Try to find the inbound that has this email
	// We'll match by email in the client_traffic table
	_, err := s.db.Exec(`
INSERT INTO client_traffic (inbound_id, email, up, down, total)
VALUES (0, ?, ?, ?, ?)
ON CONFLICT(inbound_id, email) DO UPDATE SET
    up = excluded.up,
    down = excluded.down,
    total = excluded.total`,
		email, up, down, up+down)
	return err
}

func (s *Store) ResetClientTraffic(inboundID int64, email string) error {
	_, err := s.db.Exec(`
UPDATE client_traffic
SET up = 0, down = 0, total = 0
WHERE inbound_id = ? AND email = ?`, inboundID, strings.TrimSpace(email))
	return err
}

func (s *Store) ResetAllClientTraffic(inboundID int64) error {
	_, err := s.db.Exec(`
UPDATE client_traffic
SET up = 0, down = 0, total = 0
WHERE inbound_id = ?`, inboundID)
	return err
}
