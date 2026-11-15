package entity

import "time"

// RefreshToken is one long-lived token of a sign-in session. Only a SHA-256
// hash of the token is stored. Tokens are rotated on every use, and every
// token created from the same login shares a FamilyID, so a whole session can
// be revoked at once (logout, or a stolen token being replayed).
type RefreshToken struct {
	BaseEntity
	UserID       string     `gorm:"type:varchar(255);not null;index"`
	FamilyID     string     `gorm:"type:varchar(255);not null;index"`
	TokenHash    string     `gorm:"type:varchar(64);not null;uniqueIndex"`
	ExpiresAt    time.Time  `gorm:"not null;index"`
	RevokedAt    *time.Time `gorm:"index"`
	ReplacedByID string     `gorm:"type:varchar(255)"`
}
