package user

import (
	"context"

	exUser "donetick.com/core/external/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type ExtendedUserRepository struct {
	db *gorm.DB
}

func NewExtendedUserRepository(db *gorm.DB) *ExtendedUserRepository {
	return &ExtendedUserRepository{db}
}

func (a *ExtendedUserRepository) FindFullUserByEmail(ctx context.Context, email string) (*exUser.UserExtended, error) {
	logger := logging.FromContext(ctx)
	logger.Debugw("repository.user.FindFullUserByEmail", "email", email)
	var acc exUser.UserExtended
	if err := a.db.Table("users").
		Select("users.*, s.expired_at, s.status , s.customer_id").
		Joins("LEFT JOIN stripe_customers sc ON sc.user_id = users.id").
		Joins("LEFT JOIN stripe_subscriptions s ON s.customer_id = sc.customer_id AND s.expired_at > now() OR s.expired_at is null").
		Where("email = ?", email).
		First(&acc).Error; err != nil {
		logger.Error("repository.user.FindFullUserByEmail failed to find", "err", err)
		return nil, err
	}
	return &acc, nil
}

func (a *ExtendedUserRepository) FindFullUserByUsername(ctx context.Context, username string) (*exUser.UserExtended, error) {
	logger := logging.FromContext(ctx)
	logger.Debugw("repository.user.FindFullUserByUsername", "username", username)
	var acc exUser.UserExtended
	if err := a.db.Table("users").
		Select("users.*, s.expired_at, s.status , s.customer_id").
		Joins("LEFT JOIN stripe_customers sc ON sc.user_id = users.id").
		Joins("LEFT JOIN stripe_subscriptions s ON s.customer_id = sc.customer_id AND s.expired_at > now() OR s.expired_at is null").
		Where("username = ?", username).
		First(&acc).Error; err != nil {
		logger.Error("repository.user.FindFullUserByUsername failed to find", "err", err)
		return nil, err
	}
	return &acc, nil
}
