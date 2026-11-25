package user

import (
	"context"
	uModel "donetick.com/core/internal/user/model"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type APITokenOptions struct {
	Scopes    []string
	ExpiresAt *time.Time
	CircleID  *int
}

func (r *UserRepository) GetAPIToken(ctx context.Context, raw string) (*uModel.APIToken, error) {
	prefixed := strings.HasPrefix(raw, uModel.APITokenPrefix)
	if prefixed {
		raw = hashAPIToken(raw)
	}
	var token uModel.APIToken
	if err := r.db.WithContext(ctx).Where("token = ?", raw).First(&token).Error; err != nil {
		return nil, err
	}
	if token.Prefix != nil && *token.Prefix == uModel.APITokenPrefix && !prefixed {
		return nil, errors.New("invalid API token format")
	}
	if token.RevokedAt != nil || (token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now().UTC())) {
		return nil, errors.New("API token expired or revoked")
	}
	return &token, nil
}

func (r *UserRepository) UpdateAPIToken(ctx context.Context, userID int, tokenID string, opts APITokenOptions) error {
	// Model update preserves the token secret and creation date. Revoked tokens cannot be revived.
	result := r.db.WithContext(ctx).Model(&uModel.APIToken{}).Where("id = ? AND user_id = ? AND revoked_at IS NULL", tokenID, userID).Updates(map[string]interface{}{"scopes": mustEncodeScopes(opts.Scopes), "expires_at": opts.ExpiresAt, "circle_id": opts.CircleID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("token not found")
	}
	return nil
}

func mustEncodeScopes(scopes []string) string {
	data, _ := json.Marshal(scopes) // strings always marshal successfully
	return string(data)
}

func (r *UserRepository) TouchAPIToken(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Model(&uModel.APIToken{}).Where("id = ?", id).Update("last_used_at", time.Now().UTC()).Error
}
