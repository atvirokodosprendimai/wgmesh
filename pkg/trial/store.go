package trial

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based trial store
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "trials.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &SQLiteStore{
		db:   db,
		path: dbPath,
	}

	if err := store.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing store: %w", err)
	}

	return store, nil
}

// init creates the database schema
func (s *SQLiteStore) init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS trials (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			source TEXT,
			created_at DATETIME,
			status TEXT DEFAULT 'pending'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trials_email ON trials(email)`,
		`CREATE TABLE IF NOT EXISTS trial_email_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trial_id TEXT,
			tracking_id TEXT,
			sent_at DATETIME,
			FOREIGN KEY (trial_id) REFERENCES trials(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_email_log_trial ON trial_email_log(trial_id)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("executing schema query: %w", err)
		}
	}

	return nil
}

// Create stores a new trial
func (s *SQLiteStore) Create(trial *Trial) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO trials (id, email, source, created_at, status) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, trial.ID, trial.Email, trial.Source, trial.CreatedAt, trial.Status)
	if err != nil {
		return fmt.Errorf("inserting trial: %w", err)
	}

	return nil
}

// Exists checks if an email is already registered
func (s *SQLiteStore) Exists(email string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM trials WHERE email = ?)`
	err := s.db.QueryRow(query, email).Scan(&exists)
	return err == nil && exists
}

// GetByID retrieves a trial by ID
func (s *SQLiteStore) GetByID(id string) (*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var trial Trial
	query := `SELECT id, email, source, created_at, status FROM trials WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(&trial.ID, &trial.Email, &trial.Source, &trial.CreatedAt, &trial.Status)
	if err != nil {
		return nil, fmt.Errorf("querying trial: %w", err)
	}

	return &trial, nil
}

// GetByEmail retrieves a trial by email
func (s *SQLiteStore) GetByEmail(email string) (*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var trial Trial
	query := `SELECT id, email, source, created_at, status FROM trials WHERE email = ?`
	err := s.db.QueryRow(query, email).Scan(&trial.ID, &trial.Email, &trial.Source, &trial.CreatedAt, &trial.Status)
	if err != nil {
		return nil, fmt.Errorf("querying trial by email: %w", err)
	}

	return &trial, nil
}

// UpdateStatus updates a trial's status
func (s *SQLiteStore) UpdateStatus(trialID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE trials SET status = ? WHERE id = ?`
	_, err := s.db.Exec(query, status, trialID)
	if err != nil {
		return fmt.Errorf("updating trial status: %w", err)
	}

	return nil
}

// MarkEmailSent records that an email was sent for a trial
func (s *SQLiteStore) MarkEmailSent(trialID string, trackingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO trial_email_log (trial_id, tracking_id, sent_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, trialID, trackingID, time.Now())
	if err != nil {
		return fmt.Errorf("inserting email log: %w", err)
	}

	return nil
}

// EmailSent checks if an email was already sent for a trial
func (s *SQLiteStore) EmailSent(trialID string, trackingID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM trial_email_log WHERE trial_id = ? AND tracking_id = ?)`
	err := s.db.QueryRow(query, trialID, trackingID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking email sent: %w", err)
	}

	return exists, nil
}

// GetPendingNurture retrieves trials that may need nurture emails
func (s *SQLiteStore) GetPendingNurture() ([]*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, email, source, created_at, status FROM trials WHERE status NOT IN ('expired', 'unsubscribed') ORDER BY created_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying pending trials: %w", err)
	}
	defer rows.Close()

	var trials []*Trial
	for rows.Next() {
		var trial Trial
		if err := rows.Scan(&trial.ID, &trial.Email, &trial.Source, &trial.CreatedAt, &trial.Status); err != nil {
			return nil, fmt.Errorf("scanning trial: %w", err)
		}
		trials = append(trials, &trial)
	}

	return trials, nil
}

// GetExpiring retrieves trials expiring before the given time
func (s *SQLiteStore) GetExpiring(before time.Time) ([]*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Trials expire 14 days after creation
	cutoff := before.Add(-14 * 24 * time.Hour)
	query := `SELECT id, email, source, created_at, status FROM trials WHERE created_at < ? AND status = 'pending'`
	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying expiring trials: %w", err)
	}
	defer rows.Close()

	var trials []*Trial
	for rows.Next() {
		var trial Trial
		if err := rows.Scan(&trial.ID, &trial.Email, &trial.Source, &trial.CreatedAt, &trial.Status); err != nil {
			return nil, fmt.Errorf("scanning trial: %w", err)
		}
		trials = append(trials, &trial)
	}

	return trials, nil
}

// GetExpired retrieves trials that have expired
func (s *SQLiteStore) GetExpired(before time.Time) ([]*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Trials expire 14 days after creation
	cutoff := before.Add(-14 * 24 * time.Hour)
	query := `SELECT id, email, source, created_at, status FROM trials WHERE created_at < ? AND status = 'pending'`
	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying expired trials: %w", err)
	}
	defer rows.Close()

	var trials []*Trial
	for rows.Next() {
		var trial Trial
		if err := rows.Scan(&trial.ID, &trial.Email, &trial.Source, &trial.CreatedAt, &trial.Status); err != nil {
			return nil, fmt.Errorf("scanning trial: %w", err)
		}
		trials = append(trials, &trial)
	}

	return trials, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// FileStore implements Store using JSON file storage (simpler alternative)
type FileStore struct {
	path   string
	trials map[string]*Trial
	logs   map[string][]*EmailLog
	mu     sync.RWMutex
}

// NewFileStore creates a new file-based trial store
func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:   path,
		trials: make(map[string]*Trial),
		logs:   make(map[string][]*EmailLog),
	}

	if err := store.load(); err != nil {
		return nil, fmt.Errorf("loading store: %w", err)
	}

	return store, nil
}

// load reads existing data from disk
func (s *FileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // First run, no data yet
		}
		return fmt.Errorf("reading store: %w", err)
	}

	var stored struct {
		Trials map[string]*Trial      `json:"trials"`
		Logs   map[string][]*EmailLog `json:"logs"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("unmarshaling store: %w", err)
	}

	s.trials = stored.Trials
	s.logs = stored.Logs

	return nil
}

// save writes data to disk atomically
func (s *FileStore) save() error {
	tmpPath := s.path + ".tmp"

	data, err := json.MarshalIndent(struct {
		Trials map[string]*Trial      `json:"trials"`
		Logs   map[string][]*EmailLog `json:"logs"`
	}{
		Trials: s.trials,
		Logs:   s.logs,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling store: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// Create stores a new trial
func (s *FileStore) Create(trial *Trial) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.trials[trial.ID] = trial
	if s.logs[trial.ID] == nil {
		s.logs[trial.ID] = make([]*EmailLog, 0)
	}

	return s.save()
}

// Exists checks if an email is already registered
func (s *FileStore) Exists(email string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.trials {
		if t.Email == email {
			return true
		}
	}
	return false
}

// GetByID retrieves a trial by ID
func (s *FileStore) GetByID(id string) (*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trial, ok := s.trials[id]
	if !ok {
		return nil, fmt.Errorf("trial not found")
	}

	return trial, nil
}

// GetByEmail retrieves a trial by email
func (s *FileStore) GetByEmail(email string) (*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.trials {
		if t.Email == email {
			return t, nil
		}
	}

	return nil, fmt.Errorf("trial not found")
}

// UpdateStatus updates a trial's status
func (s *FileStore) UpdateStatus(trialID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trial, ok := s.trials[trialID]
	if !ok {
		return fmt.Errorf("trial not found")
	}

	trial.Status = status
	return s.save()
}

// MarkEmailSent records that an email was sent for a trial
func (s *FileStore) MarkEmailSent(trialID string, trackingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log := &EmailLog{
		TrialID:    trialID,
		TrackingID: trackingID,
		SentAt:     time.Now(),
	}

	s.logs[trialID] = append(s.logs[trialID], log)
	return s.save()
}

// EmailSent checks if an email was already sent for a trial
func (s *FileStore) EmailSent(trialID string, trackingID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs, ok := s.logs[trialID]
	if !ok {
		return false, nil
	}

	for _, log := range logs {
		if log.TrackingID == trackingID {
			return true, nil
		}
	}

	return false, nil
}

// GetPendingNurture retrieves trials that may need nurture emails
func (s *FileStore) GetPendingNurture() ([]*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var trials []*Trial
	for _, t := range s.trials {
		if t.Status != "expired" && t.Status != "unsubscribed" {
			trials = append(trials, t)
		}
	}

	return trials, nil
}

// GetExpiring retrieves trials expiring before the given time
func (s *FileStore) GetExpiring(before time.Time) ([]*Trial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := before.Add(-14 * 24 * time.Hour)
	var trials []*Trial

	for _, t := range s.trials {
		if t.CreatedAt.Before(cutoff) && t.Status == "pending" {
			trials = append(trials, t)
		}
	}

	return trials, nil
}

// GetExpired retrieves trials that have expired
func (s *FileStore) GetExpired(before time.Time) ([]*Trial, error) {
	return s.GetExpiring(before)
}
