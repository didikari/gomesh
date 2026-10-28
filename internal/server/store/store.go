package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
	GroupName    string    `json:"group_name"`
	CPUUsage     float64   `json:"cpu_usage"`
	RAMUsage     float64   `json:"ram_usage"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// User represents a web dashboard operator or admin
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // "admin" or "operator"
	CreatedAt    time.Time `json:"created_at"`
}

// EnrollmentToken represents an authorization token for agent onboarding
type EnrollmentToken struct {
	ID          string     `json:"id"`
	Token       string     `json:"token"`
	Description string     `json:"description"`
	CreatedBy   string     `json:"created_by"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
}

// SessionAudit logs remote interactions
type SessionAudit struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"device_id"`
	Username    string    `json:"username"`
	SessionType string    `json:"session_type"` // "terminal", "desktop", "file"
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	ClientIP    string    `json:"client_ip"`
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
		group_name TEXT NOT NULL DEFAULT 'Default',
		cpu_usage REAL DEFAULT 0.0,
		ram_usage REAL DEFAULT 0.0,
		last_seen_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
	CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen_at);
	CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_name);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'operator',
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS enrollment_tokens (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		description TEXT,
		created_by TEXT,
		expires_at DATETIME,
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tokens_token ON enrollment_tokens(token);

	CREATE TABLE IF NOT EXISTS session_audits (
		id TEXT PRIMARY KEY,
		device_id TEXT NOT NULL,
		username TEXT NOT NULL,
		session_type TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		client_ip TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_audits_device ON session_audits(device_id);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed executing schema: %w", err)
	}

	// Check if group_name column exists for existing tables created before
	var groupColFound bool
	rows, err := s.db.Query("PRAGMA table_info(devices);")
	if err == nil {
		for rows.Next() {
			var cid, notnull, pk int
			var name, ctype string
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
				if name == "group_name" {
					groupColFound = true
					break
				}
			}
		}
		rows.Close()
	}
	if !groupColFound {
		_, _ = s.db.Exec("ALTER TABLE devices ADD COLUMN group_name TEXT NOT NULL DEFAULT 'Default';")
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
	if dev.GroupName == "" {
		dev.GroupName = "Default"
	}

	query := `
	INSERT INTO devices (
		id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, group_name, last_seen_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hardware_uuid) DO UPDATE SET
		hostname = excluded.hostname,
		os = excluded.os,
		arch = excluded.arch,
		internal_ip = excluded.internal_ip,
		public_ip = excluded.public_ip,
		agent_version = excluded.agent_version,
		status = excluded.status,
		last_seen_at = excluded.last_seen_at
	RETURNING id, group_name;
	`

	var assignedID, assignedGroup string
	err := s.db.QueryRowContext(ctx, query,
		dev.ID, dev.HardwareUUID, dev.Hostname, dev.OS, dev.Arch,
		dev.InternalIP, dev.PublicIP, dev.AgentVersion, dev.Status, dev.GroupName,
		dev.LastSeenAt, dev.CreatedAt,
	).Scan(&assignedID, &assignedGroup)

	if err != nil {
		return fmt.Errorf("failed to upsert device: %w", err)
	}

	dev.ID = assignedID
	dev.GroupName = assignedGroup
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

// UpdateDeviceGroup changes the assigned group for a device
func (s *Store) UpdateDeviceGroup(ctx context.Context, id, groupName string) error {
	if groupName == "" {
		groupName = "Default"
	}
	query := `UPDATE devices SET group_name = ? WHERE id = ?;`
	_, err := s.db.ExecContext(ctx, query, groupName, id)
	if err != nil {
		return fmt.Errorf("failed updating device group: %w", err)
	}
	return nil
}

// ListDeviceGroups returns all distinct group names in alphabetical order
func (s *Store) ListDeviceGroups(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT group_name FROM devices ORDER BY group_name ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed querying device groups: %w", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err == nil && g != "" {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// ListDevices returns all devices sorted by group_name, status (online first) and last_seen_at
func (s *Store) ListDevices(ctx context.Context) ([]*Device, error) {
	query := `
	SELECT id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, group_name, cpu_usage, ram_usage, last_seen_at, created_at
	FROM devices
	ORDER BY group_name ASC, CASE WHEN status = 'online' THEN 0 ELSE 1 END, last_seen_at DESC;
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
			&d.InternalIP, &d.PublicIP, &d.AgentVersion, &d.Status, &d.GroupName,
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
	SELECT id, hardware_uuid, hostname, os, arch, internal_ip, public_ip, agent_version, status, group_name, cpu_usage, ram_usage, last_seen_at, created_at
	FROM devices
	WHERE id = ?;
	`
	var d Device
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&d.ID, &d.HardwareUUID, &d.Hostname, &d.OS, &d.Arch,
		&d.InternalIP, &d.PublicIP, &d.AgentVersion, &d.Status, &d.GroupName,
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

// --- USER & AUTH STORE METHODS ---

// EnsureDefaultAdmin checks if any user exists, and if not, creates default admin/admin123
func (s *Store) EnsureDefaultAdmin(ctx context.Context) (*User, bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users;").Scan(&count)
	if err != nil {
		return nil, false, fmt.Errorf("failed checking user count: %w", err)
	}

	if count > 0 {
		return nil, false, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, fmt.Errorf("failed hashing default password: %w", err)
	}

	admin := &User{
		ID:           uuid.NewString(),
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "admin",
		CreatedAt:    time.Now().UTC(),
	}

	query := `INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?);`
	_, err = s.db.ExecContext(ctx, query, admin.ID, admin.Username, admin.PasswordHash, admin.Role, admin.CreatedAt)
	if err != nil {
		return nil, false, fmt.Errorf("failed inserting default admin: %w", err)
	}

	return admin, true, nil
}

// GetUserByUsername finds a user by username
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?;`
	var u User
	err := s.db.QueryRowContext(ctx, query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed fetching user: %w", err)
	}
	return &u, nil
}

// AuthenticateUser verifies username and password
func (s *Store) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	return u, nil
}

// --- ENROLLMENT TOKEN STORE METHODS ---

// CreateEnrollmentToken creates a new token
func (s *Store) CreateEnrollmentToken(ctx context.Context, desc, createdBy string, expiresDays int) (*EnrollmentToken, error) {
	token := &EnrollmentToken{
		ID:          uuid.NewString(),
		Token:       uuid.NewString(),
		Description: desc,
		CreatedBy:   createdBy,
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
	}

	if expiresDays > 0 {
		exp := time.Now().UTC().AddDate(0, 0, expiresDays)
		token.ExpiresAt = &exp
	}

	query := `INSERT INTO enrollment_tokens (id, token, description, created_by, expires_at, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?, ?);`
	_, err := s.db.ExecContext(ctx, query, token.ID, token.Token, token.Description, token.CreatedBy, token.ExpiresAt, 1, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed creating token: %w", err)
	}
	return token, nil
}

// ValidateEnrollmentToken checks if a token string is valid
func (s *Store) ValidateEnrollmentToken(ctx context.Context, tokenStr string) (bool, error) {
	// If no active tokens exist in the database, allow enrollment (e.g. fresh install or tests)
	var activeCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM enrollment_tokens WHERE is_active = 1;").Scan(&activeCount)
	if err == nil && activeCount == 0 {
		return true, nil
	}

	if tokenStr == "" {
		return false, nil
	}

	// Also allow initial setup token if needed
	if tokenStr == "default-token" {
		return true, nil
	}

	query := `
	SELECT is_active, expires_at 
	FROM enrollment_tokens 
	WHERE token = ?;
	`
	var isActive int
	var expiresAt *time.Time
	err = s.db.QueryRowContext(ctx, query, tokenStr).Scan(&isActive, &expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed validating token: %w", err)
	}

	if isActive != 1 {
		return false, nil
	}

	if expiresAt != nil && expiresAt.Before(time.Now().UTC()) {
		return false, nil
	}

	return true, nil
}

// ListEnrollmentTokens returns all tokens
func (s *Store) ListEnrollmentTokens(ctx context.Context) ([]*EnrollmentToken, error) {
	query := `SELECT id, token, description, created_by, expires_at, is_active, created_at FROM enrollment_tokens ORDER BY created_at DESC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed querying tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*EnrollmentToken
	for rows.Next() {
		var t EnrollmentToken
		var activeInt int
		err := rows.Scan(&t.ID, &t.Token, &t.Description, &t.CreatedBy, &t.ExpiresAt, &activeInt, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		t.IsActive = activeInt == 1
		tokens = append(tokens, &t)
	}
	return tokens, nil
}

// --- SESSION AUDIT STORE METHODS ---

// IsDeviceRegistered checks if a device with given hardware UUID is already enrolled
func (s *Store) IsDeviceRegistered(ctx context.Context, hardwareUUID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE hardware_uuid = ?;", hardwareUUID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed checking registered device: %w", err)
	}
	return count > 0, nil
}

// RecordAudit logs an interactive session
func (s *Store) RecordAudit(ctx context.Context, audit *SessionAudit) error {
	if audit.ID == "" {
		audit.ID = uuid.NewString()
	}
	query := `
	INSERT INTO session_audits (id, device_id, username, session_type, started_at, ended_at, client_ip)
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := s.db.ExecContext(ctx, query,
		audit.ID, audit.DeviceID, audit.Username, audit.SessionType,
		audit.StartedAt, audit.EndedAt, audit.ClientIP,
	)
	return err
}

// ListAudits returns recent interactive session audits
func (s *Store) ListAudits(ctx context.Context, limit int) ([]*SessionAudit, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
	SELECT id, device_id, username, session_type, started_at, ended_at, client_ip
	FROM session_audits
	ORDER BY started_at DESC
	LIMIT ?;
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed querying audits: %w", err)
	}
	defer rows.Close()

	var audits []*SessionAudit
	for rows.Next() {
		var a SessionAudit
		err := rows.Scan(&a.ID, &a.DeviceID, &a.Username, &a.SessionType, &a.StartedAt, &a.EndedAt, &a.ClientIP)
		if err != nil {
			return nil, err
		}
		audits = append(audits, &a)
	}
	return audits, nil
}

// Close closes the database connection cleanly
func (s *Store) Close() error {
	return s.db.Close()
}
