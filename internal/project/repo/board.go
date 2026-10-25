package repo

import (
	"context"
	"errors"

	chModel "donetick.com/core/internal/chore/model"
	"donetick.com/core/internal/project/access"
	pModel "donetick.com/core/internal/project/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrSectionNotFound hides whether a section exists from users who can't see
	// its project, same reasoning as ErrProjectNotFound.
	ErrSectionNotFound = errors.New("section not found")
	// ErrLastSection rejects deleting a project's only section; a board always
	// needs somewhere to place its chores.
	ErrLastSection = errors.New("cannot delete the last section")
	// ErrInvalidView rejects an unknown view preference.
	ErrInvalidView = errors.New("invalid project view")
	// ErrChoreNotFound rejects moving a chore that is not in the given project.
	ErrChoreNotFound = errors.New("chore not found in project")
)

// viewableProject loads a project the caller claims to act on and checks the
// user can see it. It returns ErrProjectNotFound both when the project truly
// does not exist and when the user cannot see it, so existence never leaks.
func (r *ProjectRepository) viewableProject(ctx context.Context, tx *gorm.DB, projectID, circleID, userID int) (*pModel.Project, error) {
	return r.loadViewableProject(ctx, tx, projectID, circleID, userID, false)
}

// lockedViewableProject is viewableProject with the project row locked, which
// serializes the board writes of one project against each other and against the
// chore repository's project-scoped writes. Every mutating path must use it:
// taking the project lock first is the same order EditChore and
// CreateChoreWithAssignees use (project, then chore rows), so board moves cannot
// deadlock against them or against one another, and a section cannot be deleted
// out from under a chore that is being moved into it.
func (r *ProjectRepository) lockedViewableProject(ctx context.Context, tx *gorm.DB, projectID, circleID, userID int) (*pModel.Project, error) {
	return r.loadViewableProject(ctx, tx, projectID, circleID, userID, true)
}

func (r *ProjectRepository) loadViewableProject(ctx context.Context, tx *gorm.DB, projectID, circleID, userID int, lock bool) (*pModel.Project, error) {
	query := tx.WithContext(ctx)
	if lock {
		query = lockedQuery(ctx, tx)
	}
	var project pModel.Project
	if err := query.Where("id = ? AND circle_id = ?", projectID, circleID).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	if err := access.LoadMembers(ctx, tx, &project); err != nil {
		return nil, err
	}
	if !project.CanView(userID) {
		return nil, ErrProjectNotFound
	}
	return &project, nil
}

// lockedQuery starts a query that takes a row lock on Postgres; sqlite has no
// row-level locking but a transaction already serializes writers there.
func lockedQuery(ctx context.Context, tx *gorm.DB) *gorm.DB {
	q := tx.WithContext(ctx)
	if tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return q
}

// positioned is the (id, position) pair gapPosition needs; it is built from
// whichever row type (section or chore) is being reordered.
type positioned struct {
	ID       int
	Position int
}

// gapPosition picks a position for an entry so it sorts right after the one
// with id afterID among siblings, which must be ordered ascending by Position
// and must NOT include the entry being placed. afterID nil means "first".
//
// ok is false when afterID is non-nil but not found among siblings (the
// caller treats that as "not found" since it means the reference row is not
// actually a sibling in this scope).
//
// needsRenumber is true when there is no integer gap left between the two
// neighbors (e.g. consecutive positions 10 and 11). The caller must then
// renumber siblings to evenly spaced positions and call gapPosition again,
// which always succeeds the second time since fresh spacing is a multiple of
// 1000.
func gapPosition(siblings []positioned, afterID *int) (pos int, needsRenumber bool, ok bool) {
	prev := 0
	next := 0
	hasNext := false

	if afterID == nil {
		if len(siblings) > 0 {
			next = siblings[0].Position
			hasNext = true
		}
	} else {
		found := false
		for i, s := range siblings {
			if s.ID != *afterID {
				continue
			}
			found = true
			prev = s.Position
			if i+1 < len(siblings) {
				next = siblings[i+1].Position
				hasNext = true
			}
			break
		}
		if !found {
			return 0, false, false
		}
	}

	if !hasNext {
		return prev + 1000, false, true
	}
	mid := prev + (next-prev)/2
	if mid <= prev || mid >= next {
		return 0, true, true
	}
	return mid, false, true
}

// GetSections returns a project's board columns ordered by position. The
// caller is responsible for checking the user can see the project.
func (r *ProjectRepository) GetSections(ctx context.Context, projectID, circleID int) ([]pModel.ProjectSection, error) {
	var sections []pModel.ProjectSection
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND circle_id = ?", projectID, circleID).
		Order("position ASC").Find(&sections).Error; err != nil {
		return nil, err
	}
	return sections, nil
}

// CreateSection adds a board column to a project. Anybody who can see the
// project can manage its shared board, not only its owner.
func (r *ProjectRepository) CreateSection(ctx context.Context, projectID, circleID, userID int, name string) (*pModel.ProjectSection, error) {
	var section pModel.ProjectSection
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.lockedViewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		var maxPosition int
		if err := tx.WithContext(ctx).Model(&pModel.ProjectSection{}).
			Where("project_id = ?", projectID).
			Select("COALESCE(MAX(position), 0)").Scan(&maxPosition).Error; err != nil {
			return err
		}
		section = pModel.ProjectSection{
			ProjectID: projectID,
			CircleID:  circleID,
			Name:      name,
			Position:  maxPosition + 1000,
		}
		return tx.WithContext(ctx).Create(&section).Error
	})
	if err != nil {
		return nil, err
	}
	return &section, nil
}

// RenameSection renames a board column.
func (r *ProjectRepository) RenameSection(ctx context.Context, projectID, sectionID, circleID, userID int, name string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.viewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		res := tx.WithContext(ctx).Model(&pModel.ProjectSection{}).
			Where("id = ? AND project_id = ?", sectionID, projectID).
			Update("name", name)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrSectionNotFound
		}
		return nil
	})
}

// renumberSections spaces siblings evenly (1000, 2000, ...) in their current
// order and returns their fresh positions, so gapPosition always finds room
// afterwards. Must run inside tx, with siblings already row-locked.
func renumberSections(ctx context.Context, tx *gorm.DB, siblings []pModel.ProjectSection) ([]positioned, error) {
	result := make([]positioned, len(siblings))
	for i, s := range siblings {
		pos := (i + 1) * 1000
		if err := tx.WithContext(ctx).Model(&pModel.ProjectSection{}).Where("id = ?", s.ID).Update("position", pos).Error; err != nil {
			return nil, err
		}
		result[i] = positioned{ID: s.ID, Position: pos}
	}
	return result, nil
}

// MoveSection repositions one section to sort right after afterSectionID
// (nil for "first"), touching only that section's position row in the common
// case. It is safe for concurrent callers moving different sections: each
// move is computed from a fresh, row-locked read of the current order rather
// than from a client-supplied snapshot of the whole list.
func (r *ProjectRepository) MoveSection(ctx context.Context, projectID, sectionID int, afterSectionID *int, circleID, userID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.lockedViewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		var sections []pModel.ProjectSection
		if err := lockedQuery(ctx, tx).Where("project_id = ?", projectID).
			Order("position ASC").Find(&sections).Error; err != nil {
			return err
		}
		var moving bool
		others := make([]pModel.ProjectSection, 0, len(sections))
		for _, s := range sections {
			if s.ID == sectionID {
				moving = true
				continue
			}
			others = append(others, s)
		}
		if !moving {
			return ErrSectionNotFound
		}
		// Dropping a column onto itself asks for the position it already has.
		if afterSectionID != nil && *afterSectionID == sectionID {
			return nil
		}

		siblings := make([]positioned, len(others))
		for i, s := range others {
			siblings[i] = positioned{ID: s.ID, Position: s.Position}
		}
		pos, needsRenumber, ok := gapPosition(siblings, afterSectionID)
		if !ok {
			return ErrSectionNotFound
		}
		if needsRenumber {
			siblings, err := renumberSections(ctx, tx, others)
			if err != nil {
				return err
			}
			pos, _, ok = gapPosition(siblings, afterSectionID)
			if !ok {
				return ErrSectionNotFound
			}
		}
		return tx.WithContext(ctx).Model(&pModel.ProjectSection{}).
			Where("id = ? AND project_id = ?", sectionID, projectID).
			Update("position", pos).Error
	})
}

// DeleteSection removes a board column, moving its chores into the project's
// remaining section with the lowest position (mirroring the frontend's local
// fallback-section behavior before this endpoint existed) and bumping their
// sync versions so clients pick up the move.
func (r *ProjectRepository) DeleteSection(ctx context.Context, projectID, sectionID, circleID, userID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.lockedViewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		var sections []pModel.ProjectSection
		if err := tx.WithContext(ctx).Where("project_id = ?", projectID).
			Order("position ASC").Find(&sections).Error; err != nil {
			return err
		}
		var target *pModel.ProjectSection
		var fallback *pModel.ProjectSection
		for i := range sections {
			if sections[i].ID == sectionID {
				target = &sections[i]
				continue
			}
			if fallback == nil {
				fallback = &sections[i]
			}
		}
		if target == nil {
			return ErrSectionNotFound
		}
		if fallback == nil {
			return ErrLastSection
		}

		var maxPosition int
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
			Where("section_id = ?", fallback.ID).
			Select("COALESCE(MAX(board_position), 0)").Scan(&maxPosition).Error; err != nil {
			return err
		}
		var choreIDs []int
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
			Where("section_id = ? AND project_id = ?", sectionID, projectID).
			Order("board_position ASC").Pluck("id", &choreIDs).Error; err != nil {
			return err
		}
		if len(choreIDs) > 0 {
			// One statement for the whole column instead of one per card. Shifting
			// every moved card by the same offset puts them all after the fallback
			// section's last card and keeps their relative order; cards that already
			// shared a position stay tied, exactly as they were before the move.
			if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
				Where("section_id = ? AND project_id = ?", sectionID, projectID).
				Updates(map[string]interface{}{
					"section_id":     fallback.ID,
					"board_position": gorm.Expr("board_position + ?", maxPosition+1000),
				}).Error; err != nil {
				return err
			}
		}
		if err := r.bumpChoreSyncVersions(ctx, tx, circleID, choreIDs); err != nil {
			return err
		}
		return tx.WithContext(ctx).Where("id = ? AND project_id = ?", sectionID, projectID).
			Delete(&pModel.ProjectSection{}).Error
	})
}

// renumberChores spaces a section's chores evenly (1000, 2000, ...) in their
// current order and returns their fresh positions. Must run inside tx, with
// siblings already row-locked.
func renumberChores(ctx context.Context, tx *gorm.DB, siblings []positioned) ([]positioned, error) {
	result := make([]positioned, len(siblings))
	for i, c := range siblings {
		pos := (i + 1) * 1000
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", c.ID).Update("board_position", pos).Error; err != nil {
			return nil, err
		}
		result[i] = positioned{ID: c.ID, Position: pos}
	}
	return result, nil
}

// MoveChore places one chore into targetSectionID, sorted right after
// afterChoreID (nil for "first"), touching only that chore's row in the
// common case. Like MoveSection, every move is computed from a fresh,
// row-locked read of the destination section, so two users dragging
// different cards around the same column at the same time cannot clobber
// each other the way overwriting a client-supplied full order would.
func (r *ProjectRepository) MoveChore(ctx context.Context, projectID, choreID, targetSectionID int, afterChoreID *int, circleID, userID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.lockedViewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		var sections []positioned
		if err := tx.WithContext(ctx).Model(&pModel.ProjectSection{}).
			Select("id, position").Where("project_id = ?", projectID).
			Order("position ASC").Scan(&sections).Error; err != nil {
			return err
		}
		targetExists := false
		for _, s := range sections {
			if s.ID == targetSectionID {
				targetExists = true
				break
			}
		}
		if !targetExists {
			return ErrSectionNotFound
		}
		// A chore in the project that has no section of its own is shown in the
		// project's first column (see Chore.SectionID), and clients compute
		// afterChoreID from what they show. So when the first column is the
		// destination, those chores are siblings too: leaving them out makes an
		// afterChoreID pointing at one of them look like a stranger and rejects
		// the drop.
		includesUnplaced := sections[0].ID == targetSectionID

		var moving chModel.Chore
		if err := lockedQuery(ctx, tx).Select("id, section_id").
			Where("id = ? AND project_id = ? AND circle_id = ?", choreID, projectID, circleID).
			First(&moving).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrChoreNotFound
			}
			return err
		}
		// Dropping a card onto itself asks for the position it already has.
		if afterChoreID != nil && *afterChoreID == choreID {
			alreadyThere := moving.SectionID != nil && *moving.SectionID == targetSectionID
			if alreadyThere || (moving.SectionID == nil && includesUnplaced) {
				return nil
			}
			return ErrChoreNotFound
		}

		// Only the id and the position are needed to find the gap, so this reads
		// (and row-locks) two columns per card instead of every chore column.
		// Unplaced chores sort at board_position 0, i.e. at the top, which is
		// where clients show them.
		siblingQuery := lockedQuery(ctx, tx).Model(&chModel.Chore{}).
			Select("id, board_position AS position").
			Where("project_id = ? AND id != ?", projectID, choreID)
		if includesUnplaced {
			siblingQuery = siblingQuery.Where("(section_id = ? OR section_id IS NULL)", targetSectionID)
		} else {
			siblingQuery = siblingQuery.Where("section_id = ?", targetSectionID)
		}
		var siblings []positioned
		if err := siblingQuery.Order("board_position ASC").Scan(&siblings).Error; err != nil {
			return err
		}

		pos, needsRenumber, ok := gapPosition(siblings, afterChoreID)
		if !ok {
			return ErrChoreNotFound
		}
		renumberedIDs := []int{}
		if needsRenumber {
			var err error
			siblings, err = renumberChores(ctx, tx, siblings)
			if err != nil {
				return err
			}
			for _, s := range siblings {
				renumberedIDs = append(renumberedIDs, s.ID)
			}
			pos, _, ok = gapPosition(siblings, afterChoreID)
			if !ok {
				return ErrChoreNotFound
			}
		}

		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", choreID).
			Updates(map[string]interface{}{"section_id": targetSectionID, "board_position": pos}).Error; err != nil {
			return err
		}
		return r.bumpChoreSyncVersions(ctx, tx, circleID, append(renumberedIDs, choreID))
	})
}

// GetViewPreference returns a user's chosen layout for a project, defaulting
// to the board view when they have never set one.
func (r *ProjectRepository) GetViewPreference(ctx context.Context, projectID, circleID, userID int) (pModel.ProjectView, error) {
	if _, err := r.viewableProject(ctx, r.db, projectID, circleID, userID); err != nil {
		return "", err
	}
	var pref pModel.ProjectViewPreference
	err := r.db.WithContext(ctx).Where("project_id = ? AND user_id = ?", projectID, userID).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pModel.ProjectViewBoard, nil
	}
	if err != nil {
		return "", err
	}
	return pref.View, nil
}

// GetViewPreferences returns a user's chosen layout for each of projectIDs,
// defaulting to the board view for any project without a saved preference.
// It does not check viewability itself: the caller is expected to have
// already filtered projectIDs to ones the user can see (e.g. the result of
// GetCircleProjects), same as GetSections.
func (r *ProjectRepository) GetViewPreferences(ctx context.Context, userID int, projectIDs []int) (map[int]pModel.ProjectView, error) {
	views := make(map[int]pModel.ProjectView, len(projectIDs))
	for _, id := range projectIDs {
		views[id] = pModel.ProjectViewBoard
	}
	if len(projectIDs) == 0 {
		return views, nil
	}
	var prefs []pModel.ProjectViewPreference
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND project_id IN ?", userID, projectIDs).
		Find(&prefs).Error; err != nil {
		return nil, err
	}
	for _, pref := range prefs {
		views[pref.ProjectID] = pref.View
	}
	return views, nil
}

// SetViewPreference stores a user's own, private choice of layout for a
// project. It never affects what any other user sees.
func (r *ProjectRepository) SetViewPreference(ctx context.Context, projectID, circleID, userID int, view pModel.ProjectView) error {
	if !view.Valid() {
		return ErrInvalidView
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.viewableProject(ctx, tx, projectID, circleID, userID); err != nil {
			return err
		}
		return tx.WithContext(ctx).Exec(`
			INSERT INTO project_view_preferences (project_id, user_id, view, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (project_id, user_id) DO UPDATE SET view = excluded.view, updated_at = excluded.updated_at`,
			projectID, userID, view).Error
	})
}
