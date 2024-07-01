package user

import (
	"context"
	"fmt"
	"time"

	"donetick.com/core/config"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type IUserRepository interface {
	GetUserByUsername(username string) (*uModel.User, error)
	GetUser(id int) (*uModel.User, error)
	GetAllUsers() ([]*uModel.User, error)
	CreateUser(user *uModel.User) error
	UpdateUser(user *uModel.User) error
	UpdateUserCircle(userID, circleID int) error
	FindByEmail(email string) (*uModel.User, error)
}

type UserRepository struct {
	db               *gorm.DB
	isDonetickDotCom bool
}

func NewUserRepository(db *gorm.DB, cfg *config.Config) *UserRepository {
	return &UserRepository{db, cfg.IsDoneTickDotCom}
}

func (r *UserRepository) GetAllUsers(c context.Context, circleID int) ([]*uModel.User, error) {
	var users []*uModel.User
	if err := r.db.WithContext(c).Where("circle_id = ?", circleID).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) GetAllUsersForSystemOnly(c context.Context) ([]*uModel.User, error) {
	var users []*uModel.User
	if err := r.db.WithContext(c).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
func (r *UserRepository) CreateUser(c context.Context, user *uModel.User) (*uModel.User, error) {
	if err := r.db.WithContext(c).Save(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}
func (r *UserRepository) GetUserByUsername(c context.Context, username string) (*uModel.User, error) {
	var user *uModel.User
	if r.isDonetickDotCom {
		if err := r.db.WithContext(c).Table("users u").Select("u.*, ss.status as  subscription, ss.expired_at as expiration").Joins("left join stripe_customers sc on sc.user_id = u.id ").Joins("left join stripe_subscriptions ss on sc.customer_id = ss.customer_id").Where("username = ?", username).First(&user).Error; err != nil {
			return nil, err
		}
	} else {
		if err := r.db.WithContext(c).Table("users u").Select("u.*, 'active' as  subscription, '2999-12-31' as expiration").Where("username = ?", username).First(&user).Error; err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (r *UserRepository) UpdateUser(c context.Context, user *uModel.User) error {
	return r.db.WithContext(c).Save(user).Error
}

func (r *UserRepository) UpdateUserCircle(c context.Context, userID, circleID int) error {
	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("circle_id", circleID).Error
}

func (r *UserRepository) FindByEmail(c context.Context, email string) (*uModel.User, error) {
	var user *uModel.User
	if err := r.db.WithContext(c).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) SetPasswordResetToken(c context.Context, email, token string) error {
	// confirm user exists with email:
	user, err := r.FindByEmail(c, email)
	if err != nil {
		return err
	}
	// save new token:
	if err := r.db.WithContext(c).Model(&uModel.UserPasswordReset{}).Save(&uModel.UserPasswordReset{
		UserID:         user.ID,
		Token:          token,
		Email:          email,
		ExpirationDate: time.Now().UTC().Add(time.Hour * 24),
	}).Error; err != nil {
		return err
	}
	return nil
}

func (r *UserRepository) UpdatePasswordByToken(ctx context.Context, email string, token string, password string) error {
	logger := logging.FromContext(ctx)

	logger.Debugw("account.db.UpdatePasswordByToken", "email", email)
	upr := &uModel.UserPasswordReset{
		Email: email,
		Token: token,
	}
	result := r.db.WithContext(ctx).Where("email = ?", email).Where("token = ?", token).Delete(upr)
	if result.RowsAffected <= 0 {
		return fmt.Errorf("invalid token")
	}
	// find account by email and update password:
	chain := r.db.WithContext(ctx).Model(&uModel.User{}).Where("email = ?", email).UpdateColumns(map[string]interface{}{"password": password})
	if chain.Error != nil {
		return chain.Error
	}
	if chain.RowsAffected == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *UserRepository) StoreAPIToken(c context.Context, userID int, name string, tokenCode string) (*uModel.APIToken, error) {
	token := &uModel.APIToken{
		UserID:    userID,
		Name:      name,
		Token:     tokenCode,
		CreatedAt: time.Now().UTC(),
	}
	if err := r.db.WithContext(c).Model(&uModel.APIToken{}).Save(
		token).Error; err != nil {
		return nil, err

	}
	return token, nil
}

func (r *UserRepository) GetUserByToken(c context.Context, token string) (*uModel.User, error) {
	var user *uModel.User
	if err := r.db.WithContext(c).Table("users u").Select("u.*").Joins("left join api_tokens at on at.user_id = u.id").Where("at.token = ?", token).First(&user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetAllUserTokens(c context.Context, userID int) ([]*uModel.APIToken, error) {
	var tokens []*uModel.APIToken
	if err := r.db.WithContext(c).Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *UserRepository) DeleteAPIToken(c context.Context, userID int, tokenID string) error {
	return r.db.WithContext(c).Where("id = ? AND user_id = ?", tokenID, userID).Delete(&uModel.APIToken{}).Error
}
