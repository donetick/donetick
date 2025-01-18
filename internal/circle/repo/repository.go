package repo

import (
	"context"
	"time"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
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
	if err := r.db.WithContext(c).
		Table("user_circles uc").
		Select("uc.*, u.username, u.display_name, u.chat_id,  unt.user_id as user_id, unt.target_id as target_id, unt.type as notification_type").
		Joins("left join users u on u.id = uc.user_id").
		Joins("left join user_notification_targets unt on unt.user_id = u.id").
		Where("uc.circle_id = ?", circleID).
		Scan(&circleUsers).Error; err != nil {
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

func (r *CircleRepository) GetDefaultCircle(c context.Context, userID int) (*cModel.Circle, error) {
	var circle cModel.Circle
	if err := r.db.WithContext(c).Raw("SELECT circles.* FROM circles LEFT JOIN user_circles on circles.id = user_circles.circle_id WHERE user_circles.user_id = ? AND user_circles.role = 'admin'", userID).Scan(&circle).Error; err != nil {
		return nil, err
	}
	return &circle, nil
}

func (r *CircleRepository) AssignDefaultCircle(c context.Context, userID int) error {
	defaultCircle, err := r.GetDefaultCircle(c, userID)
	if err != nil {
		return err
	}

	return r.db.WithContext(c).Model(&uModel.User{}).Where("id = ?", userID).Update("circle_id", defaultCircle.ID).Error
}

func (r *CircleRepository) RedeemPoints(c context.Context, circleID int, userID int, points int, createdBy int) error {
	logger := logging.FromContext(c)
	err := r.db.Transaction(func(tx *gorm.DB) error {

		if err := tx.Model(&cModel.UserCircle{}).Where("user_id = ? AND circle_id = ?", userID, circleID).Update("points_redeemed", gorm.Expr("points_redeemed + ?", points)).Error; err != nil {
			return err
		}
		if err := tx.Create(&pModel.PointsHistory{
			Action:    pModel.PointsHistoryActionRedeem,
			CircleID:  circleID,
			UserID:    userID,
			Points:    points,
			CreatedAt: time.Now().UTC(),
			CreatedBy: createdBy,
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error("Error redeeming points", err)
		return err
	}
	return nil
}
