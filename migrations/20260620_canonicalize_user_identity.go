package migrations

import (
	"context"

	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CanonicalizeUserIdentity20260620 backfills canonicalisation: usernames and
// emails are now stored trimmed + lowercased (see User.BeforeSave), so legacy
// rows are rewritten to match. With the column canonical, the existing unique
// indexes enforce case-insensitive uniqueness and login lookups stay plain
// equality checks.
//
// Rows whose lowercased value would collide with another existing row are left
// untouched (lowercasing them would violate the unique index) and logged so an
// operator can merge the duplicate accounts manually. This keeps startup safe on
// instances that already accumulated case-variant duplicates.
type CanonicalizeUserIdentity20260620 struct{}

func (m CanonicalizeUserIdentity20260620) ID() string {
	return "20260620_canonicalize_user_identity"
}

func (m CanonicalizeUserIdentity20260620) Description() string {
	return `Backfill canonical (trimmed, lowercased) usernames and emails for existing users so the persisted value is case-insensitive; skip and log case-variant duplicates that can't be lowercased without violating uniqueness.`
}

func (m CanonicalizeUserIdentity20260620) Down(ctx context.Context, db *gorm.DB) error {
	// No-op: canonicalisation is not reversible (the original casing is lost).
	return nil
}

func (m CanonicalizeUserIdentity20260620) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	if err := m.canonicalizeColumn(ctx, db, log, "email"); err != nil {
		return err
	}
	if err := m.canonicalizeColumn(ctx, db, log, "username"); err != nil {
		return err
	}
	return nil
}

// canonicalizeColumn lowercases the given column for every row that does not
// collide with another row after lowercasing. Collisions are logged and skipped.
func (m CanonicalizeUserIdentity20260620) canonicalizeColumn(ctx context.Context, db *gorm.DB, log *zap.SugaredLogger, column string) error {
	// Find values that would collapse onto the same lowercase key (case-variant
	// duplicates). These can't be lowercased without breaking the unique index.
	var collisions []string
	if err := db.WithContext(ctx).Model(&uModel.User{}).
		Where(column+" != ''").
		Group("LOWER(" + column + ")").
		Having("COUNT(*) > 1").
		Pluck("LOWER("+column+")", &collisions).Error; err != nil {
		log.Errorf("canonicalize %s: failed to detect collisions: %v", column, err)
		return err
	}

	for _, key := range collisions {
		log.Warnf("canonicalize %s: skipping case-variant duplicate %q — merge these accounts manually", column, key)
	}

	q := db.WithContext(ctx).Model(&uModel.User{}).
		Where(column+" != '' AND "+column+" <> LOWER("+column+")")
	if len(collisions) > 0 {
		q = q.Where("LOWER("+column+") NOT IN ?", collisions)
	}
	if err := q.UpdateColumn(column, gorm.Expr("LOWER("+column+")")).Error; err != nil {
		log.Errorf("canonicalize %s: failed to backfill: %v", column, err)
		return err
	}
	return nil
}

func init() {
	Register(CanonicalizeUserIdentity20260620{})
}
