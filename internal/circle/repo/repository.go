package repo

import (
	"context"

	cModel "donetick.com/core/internal/circle/model"
	"gorm.io/gorm"
)

type ICircleRepository interface {
	CreateCircle(circle *cModel.Circle) error
	AddUserToCircle(circleUser *cModel.UserCircle) error
	GetCircleUsers(circleID int) ([]*cModel.UserCircle, error)
	GetUserCircles(userID int) ([]*cModel.Circle, error)
	DeleteUserFromCircle(circleID, userID int) error
	ChangeUserRole(circleID, userID int, role string) error
	GetCircleByInviteCode(inviteCode string) (*cModel.Circle, error)
	GetCircleByID(circleID int) (*cModel.Circle, error)
}

type CircleRepository struct {
	db *gorm.DB
}

func NewCircleRepository(db *gorm.DB) *CircleRepository {
	return &CircleRepository{db}
}

func (r *CircleRepository) CreateCircle(c context.Context, circle *cModel.Circle) (*cModel.Circle, error) {
	if err := r.db.WithContext(c).Save(&circle).Error; err != nil {
		return nil, err
	}
	return circle, nil

}

func (r *CircleRepository) AddUserToCircle(c context.Context, circleUser *cModel.UserCircle) error {
	return r.db.WithContext(c).Save(circleUser).Error
}

func (r *CircleRepository) GetCircleUsers(c context.Context, circleID int) ([]*cModel.UserCircleDetail, error) {
	var circleUsers []*cModel.UserCircleDetail
	// join user table to get user details like username and display name:
	if err := r.db.WithContext(c).Raw("SELECT * FROM user_circles LEFT JOIN users on users.id = user_circles.user_id WHERE user_circles.circle_id = ?", circleID).Scan(&circleUsers).Error; err != nil {
		return nil, err
	}
	return circleUsers, nil
}

func (r *CircleRepository) GetPendingJoinRequests(c context.Context, circleID int) ([]*cModel.UserCircleDetail, error) {
	var pendingRequests []*cModel.UserCircleDetail
	if err := r.db.WithContext(c).Raw("SELECT *, user_circles.id as id FROM user_circles LEFT JOIN users on users.id = user_circles.user_id WHERE user_circles.circle_id = ? AND user_circles.is_active = false", circleID).Scan(&pendingRequests).Error; err != nil {
		return nil, err
	}
	return pendingRequests, nil
}

func (r *CircleRepository) AcceptJoinRequest(c context.Context, circleID, requestID int) error {

	return r.db.WithContext(c).Model(&cModel.UserCircle{}).Where("circle_id = ? AND id = ?", circleID, requestID).Update("is_active", true).Error
}

func (r *CircleRepository) GetUserCircles(c context.Context, userID int) ([]*cModel.CircleDetail, error) {
	var circles []*cModel.CircleDetail
	if err := r.db.WithContext(c).Raw("SELECT circles.*, user_circles.role as role, user_circles.created_at uc_created_at  FROM circles Left JOIN user_circles on circles.id = user_circles.circle_id WHERE user_circles.user_id = ? ORDER BY uc_created_at desc", userID).Scan(&circles).Error; err != nil {
		return nil, err
	}
	return circles, nil
}

func (r *CircleRepository) DeleteUserFromCircle(c context.Context, circleID, userID int) error {
	return r.db.WithContext(c).Where("circle_id = ? AND user_id = ?", circleID, userID).Delete(&cModel.UserCircle{}).Error
}

func (r *CircleRepository) ChangeUserRole(c context.Context, circleID, userID int, role int) error {
	return r.db.WithContext(c).Model(&cModel.UserCircle{}).Where("circle_id = ? AND user_id = ?", circleID, userID).Update("role", role).Error
}

func (r *CircleRepository) GetCircleByInviteCode(c context.Context, inviteCode string) (*cModel.Circle, error) {
	var circle cModel.Circle
	if err := r.db.WithContext(c).Where("invite_code = ?", inviteCode).First(&circle).Error; err != nil {
		return nil, err
	}
	return &circle, nil
}

func (r *CircleRepository) GetCircleByID(c context.Context, circleID int) (*cModel.Circle, error) {
	var circle cModel.Circle
	if err := r.db.WithContext(c).First(&circle, circleID).Error; err != nil {
		return nil, err
	}
	return &circle, nil
}

func (r *CircleRepository) LeaveCircleByUserID(c context.Context, circleID, userID int) error {
	return r.db.WithContext(c).Where("circle_id = ? AND user_id = ? AND role != 'admin'", circleID, userID).Delete(&cModel.UserCircle{}).Error
}

func (r *CircleRepository) GetUserOriginalCircle(c context.Context, userID int) (int, error) {
	var circleID int
	if err := r.db.WithContext(c).Raw("SELECT circle_id FROM user_circles WHERE user_id = ? AND role = 'admin'", userID).Scan(&circleID).Error; err != nil {
		return 0, err
	}
	return circleID, nil
}

func (r *CircleRepository) DeleteMemberByID(c context.Context, circleID, userID int) error {
	return r.db.WithContext(c).Where("circle_id = ? AND user_id = ?", circleID, userID).Delete(&cModel.UserCircle{}).Error
}

func (r *CircleRepository) GetCircleAdmins(c context.Context, circleID int) ([]*cModel.UserCircleDetail, error) {
	var circleAdmins []*cModel.UserCircleDetail
	if err := r.db.WithContext(c).Raw("SELECT * FROM user_circles LEFT JOIN users on users.id = user_circles.user_id WHERE user_circles.circle_id = ? AND user_circles.role = 'admin'", circleID).Scan(&circleAdmins).Error; err != nil {
		return nil, err
	}
	return circleAdmins, nil
}
