package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Device represents a registered target machine
type Device struct {
	ID           string    `json:"id"`
	HardwareUUID string    `json:"hardware_uuid"`
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	InternalIP   string    `json:"internal_ip"`
	PublicIP     string    `json:"public_ip"`
	AgentVersion string    `json:"agent_version"`
	Status       string    `json:"status"` // "online" or "offline"
	CPUUsage     float64   `json:"cpu_usage"`
	RAMUsage     float64   `json:"ram_usage"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// Store manages database persistence using pure-Go SQLite
type Store struct {
	db *sql.DB
}

// New creates and initializes the SQLite store
func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Single writer connection pool for SQLite WAL stability
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

// migrate creates tables if not present
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		hardware_uuid TEXT UNIQUE NOT NULL,
		hostname TEXT NOT NULL,
		os TEXT NOT NULL,
		arch TEXT NOT NULL,
		internal_ip TEXT,
		public_ip TEXT,
		agent_version TEXT,
		status TEXT NOT NULL DEFAULT 'offline',
		cpu_usage REAL DEFAULT 0.0,
		ram_usage REAL DEFAULT 0.0,
		last_seen_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
	CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen_at);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed executing schema: %w", err)
	}
	return nil
}

// UpsertDevice inserts or updates device identity upon connection
func (s *Store) UpsertDevice(ctx context.Context, dev *Device) error {
	now := time.Now().UTC()
	dev.LastSeenAt = now
	if dev.CreatedAt.IsZero() {
		dev.CreatedAt = now
	}

	query := `
	INSERT INTO devices (
		id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, last_seen_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hardware_uuid) DO UPDATE SET
		hostname = excluded.hostname,
		os = excluded.os,
		arch = excluded.arch,
		internal_ip = excluded.internal_ip,
		public_ip = excluded.public_ip,
		agent_version = excluded.agent_version,
		status = excluded.status,
		last_seen_at = excluded.last_seen_at
	RETURNING id;
	`

	var assignedID string
	err := s.db.QueryRowContext(ctx, query,
		dev.ID, dev.HardwareUUID, dev.Hostname, dev.OS, dev.Arch,
		dev.InternalIP, dev.PublicIP, dev.AgentVersion, dev.Status,
		dev.LastSeenAt, dev.CreatedAt,
	).Scan(&assignedID)

	if err != nil {
		return fmt.Errorf("failed to upsert device: %w", err)
	}

	dev.ID = assignedID
	return nil
}

// UpdateHeartbeat updates telemetry and timestamp
func (s *Store) UpdateHeartbeat(ctx context.Context, deviceID string, cpu, ram float64) error {
	query := `
	UPDATE devices 
	SET cpu_usage = ?, ram_usage = ?, last_seen_at = ?, status = 'online'
	WHERE id = ?;
	`
	_, err := s.db.ExecContext(ctx, query, cpu, ram, time.Now().UTC(), deviceID)
	if err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return nil
}

// SetDeviceStatus marks a device as online or offline
func (s *Store) SetDeviceStatus(ctx context.Context, deviceID string, status string) error {
	query := `UPDATE devices SET status = ? WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, status, deviceID)
	if err != nil {
		return fmt.Errorf("failed to set device status: %w", err)
	}
	return nil
}

// ListDevices returns all devices sorted by status (online first) and last_seen_at
func (s *Store) ListDevices(ctx context.Context) ([]*Device, error) {
	query := `
	SELECT id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, cpu_usage, ram_usage, last_seen_at, created_at
	FROM devices
	ORDER BY CASE WHEN status = 'online' THEN 0 ELSE 1 END, last_seen_at DESC;
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var d Device
		err := rows.Scan(
			&d.ID, &d.HardwareUUID, &d.Hostname, &d.OS, &d.Arch,
			&d.InternalIP, &d.PublicIP, &d.AgentVersion, &d.Status,
			&d.CPUUsage, &d.RAMUsage, &d.LastSeenAt, &d.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device: %w", err)
		}
		devices = append(devices, &d)
	}

	return devices, nil
}

// GetDevice fetches a device by ID
func (s *Store) GetDevice(ctx context.Context, id string) (*Device, error) {
	query := `
	SELECT id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, cpu_usage, ram_usage, last_seen_at, created_at
	FROM devices
	WHERE id = ?;
	`
	var d Device
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&d.ID, &d.HardwareUUID, &d.Hostname, &d.OS, &d.Arch,
		&d.InternalIP, &d.PublicIP, &d.AgentVersion, &d.Status,
		&d.CPUUsage, &d.RAMUsage, &d.LastSeenAt, &d.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	return &d, nil
}

// MarkStaleDevices marks any device that has missed heartbeats as offline
func (s *Store) MarkStaleDevices(ctx context.Context, timeout time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-timeout)
	query := `
	UPDATE devices
	SET status = 'offline'
	WHERE status = 'online' AND last_seen_at < ?;
	`
	res, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed marking stale devices: %w", err)
	}
	return res.RowsAffected()
}

// Close closes the database connection cleanly
func (s *Store) Close() error {
	return s.db.Close()
}
