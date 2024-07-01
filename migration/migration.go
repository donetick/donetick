package migration

import (
	chModel "donetick.com/core/internal/chore/model"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	tModel "donetick.com/core/internal/thing/model"
	uModel "donetick.com/core/internal/user/model"
	"gorm.io/gorm"
)

func Migration(db *gorm.DB) error {
	if err := db.AutoMigrate(uModel.User{}, chModel.Chore{},
		chModel.ChoreHistory{},
		cModel.Circle{},
		cModel.UserCircle{},
		chModel.ChoreAssignees{},
		nModel.Notification{},
		uModel.UserPasswordReset{},
		tModel.Thing{},
		tModel.ThingChore{},
		tModel.ThingHistory{},
		uModel.APIToken{},
	); err != nil {
		return err
	}

	return nil
}
