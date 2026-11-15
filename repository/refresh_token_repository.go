package repository

import (
	"time"

	"gorm.io/gorm"
	"real-time-chat-app/entity"
)

type RefreshTokenRepository struct {
	Repository[entity.RefreshToken]
}

func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{}
}

func (repository RefreshTokenRepository) FindByHash(db *gorm.DB, hash string) (entity.RefreshToken, error) {
	var token entity.RefreshToken
	err := db.Where("token_hash = ?", hash).Take(&token).Error
	return token, err
}

// Rotate revokes the token if it is still active and records its successor.
// It returns false when another request already used the token.
func (repository RefreshTokenRepository) Rotate(db *gorm.DB, id, replacedByID string, now time.Time) (bool, error) {
	result := db.Model(&entity.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": now, "replaced_by_id": replacedByID})
	return result.RowsAffected == 1, result.Error
}

// RevokeFamily revokes every active token that belongs to one login.
func (repository RefreshTokenRepository) RevokeFamily(db *gorm.DB, familyID string, now time.Time) error {
	return db.Model(&entity.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", now).Error
}

// DeleteExpired removes a user's tokens that can no longer be used.
func (repository RefreshTokenRepository) DeleteExpired(db *gorm.DB, userID string, now time.Time) error {
	return db.Unscoped().
		Where("user_id = ? AND expires_at < ?", userID, now).
		Delete(&entity.RefreshToken{}).Error
}
