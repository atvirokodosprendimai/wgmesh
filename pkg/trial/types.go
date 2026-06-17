package trial

import "time"

// Trial represents a trial signup
type Trial struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // "pending", "active", "converted", "expired", "unsubscribed"
}

// EmailLog tracks email sends for a trial
type EmailLog struct {
	ID         int64     `json:"id"`
	TrialID    string    `json:"trial_id"`
	TrackingID string    `json:"tracking_id"`
	SentAt     time.Time `json:"sent_at"`
}

// Store defines the interface for trial storage
type Store interface {
	Create(trial *Trial) error
	Exists(email string) bool
	GetByID(id string) (*Trial, error)
	GetByEmail(email string) (*Trial, error)
	UpdateStatus(trialID string, status string) error
	MarkEmailSent(trialID string, trackingID string) error
	EmailSent(trialID string, trackingID string) (bool, error)
	GetPendingNurture() ([]*Trial, error)
	GetExpiring(before time.Time) ([]*Trial, error)
	GetExpired(before time.Time) ([]*Trial, error)
}
