package chore

import (
	"context"
	"errors"
	"sort"

	chModel "donetick.com/core/internal/chore/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
	"gorm.io/gorm"
)

// ErrAssigneeOutsideProject is returned when a chore in a restricted project would
// be assigned to somebody who cannot see that project.
var ErrAssigneeOutsideProject = errors.New("chores in this project can only be assigned to people who can see the project")

// ProjectAccess is the audience of a project: its visibility and, for "members",
// the users it is shared with (the owner is implied and never listed).
type ProjectAccess struct {
	Visibility pModel.ProjectVisibility `json:"visibility"`
	MemberIDs  []int                    `json:"memberIds"`
}

// AccessOf returns the current audience of a project whose members are loaded.
func AccessOf(project *pModel.Project) ProjectAccess {
	access := ProjectAccess{Visibility: project.Visibility()}
	if access.Visibility == pModel.ProjectVisibilityMembers {
		access.MemberIDs = append([]int(nil), project.MemberIDs...)
	}
	return access
}

func (a ProjectAccess) restricted() bool {
	return a.Visibility == pModel.ProjectVisibilityPrivate || a.Visibility == pModel.ProjectVisibilityMembers
}

// viewers returns who can see the project's chores when it is restricted.
func (a ProjectAccess) viewers(ownerID int) map[int]struct{} {
	set := map[int]struct{}{ownerID: {}}
	if a.Visibility == pModel.ProjectVisibilityMembers {
		for _, id := range a.MemberIDs {
			set[id] = struct{}{}
		}
	}
	return set
}

// ChoreAccessChange describes what an access change does to one chore.
type ChoreAccessChange struct {
	ChoreID          int    `json:"choreId"`
	Name             string `json:"name"`
	RemovedAssignees []int  `json:"removedAssignees,omitempty"`
	AddedAssignees   []int  `json:"addedAssignees,omitempty"`
	AssignedToBefore *int   `json:"assignedToBefore,omitempty"`
	AssignedToAfter  *int   `json:"assignedToAfter,omitempty"`
	RevokedUsers     []int  `json:"revokedUsers,omitempty"`
	GrantedUsers     []int  `json:"grantedUsers,omitempty"`

	isPrivateBefore bool
	isPrivateAfter  bool
	assigneesAfter  []int
}

// AssignmentChanged reports whether the chore's assignees or current assignee change.
func (c *ChoreAccessChange) AssignmentChanged() bool {
	return len(c.RemovedAssignees) > 0 || len(c.AddedAssignees) > 0 || !sameIntPtr(c.AssignedToBefore, c.AssignedToAfter)
}

// ProjectAccessPlan is the full, not yet applied, effect of changing a project's
// audience. The same plan backs the preview shown to the owner and the update.
type ProjectAccessPlan struct {
	ProjectID int           `json:"projectId"`
	OwnerID   int           `json:"ownerId"`
	Before    ProjectAccess `json:"before"`
	After     ProjectAccess `json:"after"`
	// Chores lists every chore of the project; most entries carry no change.
	Chores []*ChoreAccessChange `json:"chores"`
	// RevokedUsers lost sight of at least one chore, GrantedUsers gained it.
	RevokedUsers []int `json:"revokedUsers"`
	GrantedUsers []int `json:"grantedUsers"`
}

// ChangedChores returns only the chores whose assignment or audience changes.
func (p *ProjectAccessPlan) ChangedChores() []*ChoreAccessChange {
	out := make([]*ChoreAccessChange, 0)
	for _, c := range p.Chores {
		if c.AssignmentChanged() || len(c.RevokedUsers) > 0 || len(c.GrantedUsers) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// ProjectAccessResult reports what ApplyProjectAccessChange did, for the caller's
// post-commit work (realtime events, notification regeneration).
type ProjectAccessResult struct {
	Plan *ProjectAccessPlan
	// ChoreIDs whose pending notifications were dropped and must be regenerated.
	NotificationChoreIDs []int
	// Revocations maps chore ID to the users that lost it, with the sync version of
	// each user's tombstone so realtime deletions can carry it.
	Revocations map[int]map[int]int64
}

type projectChoreRow struct {
	ID             int
	Name           string
	IsPrivate      bool
	CreatedBy      int
	AssignedTo     *int
	AssignStrategy chModel.AssignmentStrategy
}

// PlanProjectAccessChange computes, without writing anything, how moving a project
// from one audience to another affects its chores.
//
// Assignments follow the audience, because a chore must not rotate or remind its
// way to somebody who can no longer see it:
//   - "only me": the upstream rule — every chore is narrowed to the owner and
//     chores assigned to "Anyone" are pinned to the owner.
//   - "members": assignees who keep access stay assigned; people who lose access
//     are removed. If that empties a chore's list, the owner takes it over rather
//     than the chore silently turning into "Anyone". A current assignee who loses
//     access is replaced by the owner (or the first remaining assignee).
//   - "circle": assignments are untouched.
//
// Completion history is never touched by an access change.
func (r *ChoreRepository) PlanProjectAccessChange(ctx context.Context, tx *gorm.DB, circleID, projectID, ownerID int, before, after ProjectAccess) (*ProjectAccessPlan, error) {
	if tx == nil {
		tx = r.db
	}
	plan := &ProjectAccessPlan{
		ProjectID:    projectID,
		OwnerID:      ownerID,
		Before:       normalizeAccess(before, ownerID),
		After:        normalizeAccess(after, ownerID),
		Chores:       []*ChoreAccessChange{},
		RevokedUsers: []int{},
		GrantedUsers: []int{},
	}

	var chores []projectChoreRow
	if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
		Select("id, name, is_private, created_by, assigned_to, assign_strategy").
		Where("project_id = ? AND circle_id = ?", projectID, circleID).
		Order("id asc").
		Scan(&chores).Error; err != nil {
		return nil, err
	}
	if len(chores) == 0 {
		return plan, nil
	}

	choreIDs := make([]int, 0, len(chores))
	for _, chore := range chores {
		choreIDs = append(choreIDs, chore.ID)
	}

	var circleUserIDs []int
	if err := tx.WithContext(ctx).Table("user_circles").Where("circle_id = ? AND is_active = ?", circleID, true).Pluck("user_id", &circleUserIDs).Error; err != nil {
		return nil, err
	}

	var assignees []chModel.ChoreAssignees
	if err := tx.WithContext(ctx).Where("chore_id IN ?", choreIDs).Order("id asc").Find(&assignees).Error; err != nil {
		return nil, err
	}
	assigneesByChore := map[int][]int{}
	for _, assignee := range assignees {
		assigneesByChore[assignee.ChoreID] = append(assigneesByChore[assignee.ChoreID], assignee.UserID)
	}

	var viewersBefore, viewersAfter map[int]struct{}
	if plan.Before.restricted() {
		viewersBefore = plan.Before.viewers(ownerID)
	}
	if plan.After.restricted() {
		viewersAfter = plan.After.viewers(ownerID)
	}

	revokedAll := map[int]struct{}{}
	grantedAll := map[int]struct{}{}
	for _, chore := range chores {
		prevAssignees := assigneesByChore[chore.ID]
		change := &ChoreAccessChange{
			ChoreID:          chore.ID,
			Name:             chore.Name,
			AssignedToBefore: copyIntPtr(chore.AssignedTo),
			AssignedToAfter:  copyIntPtr(chore.AssignedTo),
			isPrivateBefore:  chore.IsPrivate,
			isPrivateAfter:   plan.After.restricted(),
			assigneesAfter:   append([]int(nil), prevAssignees...),
		}

		switch plan.After.Visibility {
		case pModel.ProjectVisibilityPrivate:
			narrowToOwner(change, chore, prevAssignees, ownerID)
		case pModel.ProjectVisibilityMembers:
			narrowToViewers(change, chore, prevAssignees, viewersAfter, ownerID)
		}
		change.RemovedAssignees, change.AddedAssignees = diffIDs(prevAssignees, change.assigneesAfter)

		previous := viewersBefore
		if previous == nil {
			previous = choreViewerSet(&chModel.Chore{IsPrivate: chore.IsPrivate, CreatedBy: chore.CreatedBy}, prevAssignees, circleUserIDs)
		}
		next := viewersAfter
		if next == nil {
			next = choreViewerSet(&chModel.Chore{IsPrivate: false, CreatedBy: chore.CreatedBy}, change.assigneesAfter, circleUserIDs)
		}
		for id := range previous {
			if _, ok := next[id]; !ok {
				change.RevokedUsers = append(change.RevokedUsers, id)
				revokedAll[id] = struct{}{}
			}
		}
		for id := range next {
			if _, ok := previous[id]; !ok {
				change.GrantedUsers = append(change.GrantedUsers, id)
				grantedAll[id] = struct{}{}
			}
		}
		sort.Ints(change.RevokedUsers)
		sort.Ints(change.GrantedUsers)
		plan.Chores = append(plan.Chores, change)
	}
	plan.RevokedUsers = sortedKeys(revokedAll)
	plan.GrantedUsers = sortedKeys(grantedAll)
	return plan, nil
}

// narrowToOwner applies the upstream "private project" assignment rule.
func narrowToOwner(change *ChoreAccessChange, chore projectChoreRow, prevAssignees []int, ownerID int) {
	kept := make([]int, 0, 1)
	ownerAssigned := false
	for _, id := range prevAssignees {
		if id == ownerID {
			kept = append(kept, id)
			ownerAssigned = true
		}
	}
	// A chore with no assignee at all means "Anyone", which now resolves to the
	// owner — unless it is deliberately unassigned.
	if !ownerAssigned && chore.AssignStrategy != chModel.AssignmentStrategyNoAssignee {
		kept = append(kept, ownerID)
	}
	change.assigneesAfter = kept
	if chore.AssignedTo != nil && *chore.AssignedTo != ownerID {
		if chore.AssignStrategy == chModel.AssignmentStrategyNoAssignee {
			change.AssignedToAfter = nil
		} else {
			owner := ownerID
			change.AssignedToAfter = &owner
		}
	}
}

// narrowToViewers keeps every assignee who can still see the project.
func narrowToViewers(change *ChoreAccessChange, chore projectChoreRow, prevAssignees []int, viewers map[int]struct{}, ownerID int) {
	kept := make([]int, 0, len(prevAssignees))
	for _, id := range prevAssignees {
		if _, ok := viewers[id]; ok {
			kept = append(kept, id)
		}
	}
	if len(prevAssignees) > 0 && len(kept) == 0 && chore.AssignStrategy != chModel.AssignmentStrategyNoAssignee {
		kept = append(kept, ownerID)
	}
	change.assigneesAfter = kept

	if chore.AssignedTo == nil {
		return
	}
	if _, ok := viewers[*chore.AssignedTo]; ok {
		return
	}
	if chore.AssignStrategy == chModel.AssignmentStrategyNoAssignee {
		change.AssignedToAfter = nil
		return
	}
	next := ownerID
	if len(kept) > 0 && !containsInt(kept, ownerID) {
		next = kept[0]
	}
	change.AssignedToAfter = &next
}

// ApplyProjectAccessChange plans and applies an audience change for every chore of
// a project inside tx. It keeps the chore rows, their assignees, pending
// notifications and the sync stream consistent with the new audience:
//   - every chore gets a fresh sync_version (clients render project data with it),
//   - users who lose a chore get user-scoped tombstones for it and its history,
//   - users who gain a chore get its history re-versioned so delta sync sends it,
//     and their older user-scoped tombstones for it are dropped,
//   - pending notifications of changed chores are removed; the caller regenerates
//     them after commit (see ProjectAccessResult.NotificationChoreIDs).
func (r *ChoreRepository) ApplyProjectAccessChange(ctx context.Context, tx *gorm.DB, circleID, projectID, ownerID int, before, after ProjectAccess) (*ProjectAccessResult, error) {
	if tx == nil {
		tx = r.db
	}
	plan, err := r.PlanProjectAccessChange(ctx, tx, circleID, projectID, ownerID, before, after)
	if err != nil {
		return nil, err
	}
	result := &ProjectAccessResult{Plan: plan, Revocations: map[int]map[int]int64{}}
	if len(plan.Chores) == 0 {
		return result, nil
	}

	notificationChores := make([]int, 0)
	for _, change := range plan.Chores {
		if change.AssignmentChanged() || len(change.RevokedUsers) > 0 || len(change.GrantedUsers) > 0 {
			notificationChores = append(notificationChores, change.ChoreID)
		}
	}
	if len(notificationChores) > 0 {
		// Notifications contain rendered chore text and delivery targets. Remove
		// pending rows before access changes so they cannot disclose the chore after
		// this transaction commits; authorized ones are regenerated by the caller.
		if err := tx.WithContext(ctx).Where("chore_id IN ? AND is_sent = ?", notificationChores, false).
			Delete(&nModel.Notification{}).Error; err != nil {
			return nil, err
		}
	}
	result.NotificationChoreIDs = notificationChores

	// One version per chore, all from a single contiguous range.
	nextVersion, err := r.nextSyncVersionRangeWithDB(ctx, tx, circleID, len(plan.Chores))
	if err != nil {
		return nil, err
	}
	deltas := make([]visibilityDelta, 0)
	for _, change := range plan.Chores {
		if len(change.RemovedAssignees) > 0 {
			if err := tx.WithContext(ctx).Where("chore_id = ? AND user_id IN ?", change.ChoreID, change.RemovedAssignees).
				Delete(&chModel.ChoreAssignees{}).Error; err != nil {
				return nil, err
			}
		}
		for _, userID := range change.AddedAssignees {
			if err := tx.WithContext(ctx).Create(&chModel.ChoreAssignees{ChoreID: change.ChoreID, UserID: userID}).Error; err != nil {
				return nil, err
			}
		}
		fields := map[string]interface{}{"sync_version": nextVersion}
		nextVersion++
		if change.isPrivateBefore != change.isPrivateAfter {
			fields["is_private"] = change.isPrivateAfter
		}
		if !sameIntPtr(change.AssignedToBefore, change.AssignedToAfter) {
			if change.AssignedToAfter == nil {
				fields["assigned_to"] = nil
			} else {
				fields["assigned_to"] = *change.AssignedToAfter
			}
		}
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", change.ChoreID).Updates(fields).Error; err != nil {
			return nil, err
		}
		if len(change.RevokedUsers) > 0 || len(change.GrantedUsers) > 0 {
			deltas = append(deltas, visibilityDelta{choreID: change.ChoreID, revoked: change.RevokedUsers, granted: change.GrantedUsers})
		}
	}

	revocations, err := r.applyVisibilityDeltas(ctx, tx, circleID, deltas)
	if err != nil {
		return nil, err
	}
	result.Revocations = revocations
	return result, nil
}

// visibilityDelta is who lost and who gained sight of one chore.
type visibilityDelta struct {
	choreID int
	revoked []int
	granted []int
}

// applyVisibilityDeltas records the sync-side consequences of users losing or
// gaining sight of chores:
//   - a lost chore gets a user-scoped tombstone for the chore and for each of its
//     history entries, so delta-sync clients drop their cached copies of both,
//   - a gained chore gets its history entries re-versioned so delta-sync clients
//     that synced past them still receive them, and any older user-scoped
//     tombstones of that user for the chore or its history are removed, so a
//     client that syncs the revocation and the re-grant together keeps the chore.
//
// It returns, per chore and revoked user, the version of the chore tombstone.
func (r *ChoreRepository) applyVisibilityDeltas(ctx context.Context, tx *gorm.DB, circleID int, deltas []visibilityDelta) (map[int]map[int]int64, error) {
	revocations := map[int]map[int]int64{}
	if len(deltas) == 0 {
		return revocations, nil
	}

	choreIDs := make([]int, 0, len(deltas))
	for _, d := range deltas {
		choreIDs = append(choreIDs, d.choreID)
	}
	var histories []chModel.ChoreHistory
	if err := tx.WithContext(ctx).Model(&chModel.ChoreHistory{}).Select("id, chore_id").
		Where("chore_id IN ?", choreIDs).Order("id asc").Find(&histories).Error; err != nil {
		return nil, err
	}
	historiesByChore := map[int][]int{}
	for _, h := range histories {
		historiesByChore[h.ChoreID] = append(historiesByChore[h.ChoreID], h.ID)
	}

	versionsNeeded := 0
	for _, d := range deltas {
		versionsNeeded += len(d.revoked) * (1 + len(historiesByChore[d.choreID]))
		if len(d.granted) > 0 {
			versionsNeeded += len(historiesByChore[d.choreID])
		}
	}
	nextVersion := int64(0)
	if versionsNeeded > 0 {
		var err error
		nextVersion, err = r.nextSyncVersionRangeWithDB(ctx, tx, circleID, versionsNeeded)
		if err != nil {
			return nil, err
		}
	}

	tombstones := make([]syncModel.Tombstone, 0)
	for _, d := range deltas {
		historyIDs := historiesByChore[d.choreID]
		for _, userID := range d.revoked {
			user := userID
			if revocations[d.choreID] == nil {
				revocations[d.choreID] = map[int]int64{}
			}
			revocations[d.choreID][user] = nextVersion
			tombstones = append(tombstones, syncModel.Tombstone{
				CircleID: circleID, EntityType: syncModel.EntityTypeChore, EntityID: d.choreID,
				UserID: &user, SyncVersion: nextVersion,
			})
			nextVersion++
			for _, historyID := range historyIDs {
				tombstones = append(tombstones, syncModel.Tombstone{
					CircleID: circleID, EntityType: syncModel.EntityTypeChoreHistory, EntityID: historyID,
					UserID: &user, SyncVersion: nextVersion,
				})
				nextVersion++
			}
		}
		if len(d.granted) > 0 {
			for _, historyID := range historyIDs {
				if err := tx.WithContext(ctx).Model(&chModel.ChoreHistory{}).Where("id = ?", historyID).
					Update("sync_version", nextVersion).Error; err != nil {
					return nil, err
				}
				nextVersion++
			}
			if err := tx.WithContext(ctx).
				Where("circle_id = ? AND entity_type = ? AND entity_id = ? AND user_id IN ?", circleID, syncModel.EntityTypeChore, d.choreID, d.granted).
				Delete(&syncModel.Tombstone{}).Error; err != nil {
				return nil, err
			}
			if len(historyIDs) > 0 {
				if err := tx.WithContext(ctx).
					Where("circle_id = ? AND entity_type = ? AND entity_id IN ? AND user_id IN ?", circleID, syncModel.EntityTypeChoreHistory, historyIDs, d.granted).
					Delete(&syncModel.Tombstone{}).Error; err != nil {
					return nil, err
				}
			}
		}
	}
	if len(tombstones) > 0 {
		if err := tx.WithContext(ctx).CreateInBatches(&tombstones, 200).Error; err != nil {
			return nil, err
		}
	}
	return revocations, nil
}

// normalizeAccess drops the owner and duplicates from the member list and clears
// it for visibilities that have none.
func normalizeAccess(a ProjectAccess, ownerID int) ProjectAccess {
	if a.Visibility != pModel.ProjectVisibilityMembers {
		return ProjectAccess{Visibility: a.Visibility, MemberIDs: []int{}}
	}
	seen := map[int]struct{}{}
	members := make([]int, 0, len(a.MemberIDs))
	for _, id := range a.MemberIDs {
		if id == ownerID {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		members = append(members, id)
	}
	sort.Ints(members)
	return ProjectAccess{Visibility: a.Visibility, MemberIDs: members}
}

func diffIDs(before, after []int) (removed, added []int) {
	b := map[int]struct{}{}
	for _, id := range before {
		b[id] = struct{}{}
	}
	a := map[int]struct{}{}
	for _, id := range after {
		a[id] = struct{}{}
		if _, ok := b[id]; !ok {
			added = append(added, id)
		}
	}
	for _, id := range before {
		if _, ok := a[id]; !ok {
			removed = append(removed, id)
		}
	}
	sort.Ints(removed)
	sort.Ints(added)
	return removed, dedupeSorted(added)
}

func dedupeSorted(ids []int) []int {
	if len(ids) < 2 {
		return ids
	}
	out := ids[:1]
	for _, id := range ids[1:] {
		if id != out[len(out)-1] {
			out = append(out, id)
		}
	}
	return out
}

func sortedKeys(set map[int]struct{}) []int {
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func containsInt(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func copyIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// DetachProjectChores moves every chore of a project that is being deleted out of
// it. The chores keep their own privacy flag and follow the chore-level rule from
// then on (creator and assignees for private chores, the circle otherwise), so
// people who only saw them through the project lose them; they get user-scoped
// tombstones, and people who now see them get their history re-versioned.
func (r *ChoreRepository) DetachProjectChores(ctx context.Context, tx *gorm.DB, circleID int, project *pModel.Project) error {
	var chores []projectChoreRow
	if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
		Select("id, name, is_private, created_by, assigned_to, assign_strategy").
		Where("project_id = ? AND circle_id = ?", project.ID, circleID).
		Order("id asc").Scan(&chores).Error; err != nil {
		return err
	}
	if len(chores) == 0 {
		return nil
	}
	choreIDs := make([]int, 0, len(chores))
	for _, chore := range chores {
		choreIDs = append(choreIDs, chore.ID)
	}

	var circleUserIDs []int
	if err := tx.WithContext(ctx).Table("user_circles").Where("circle_id = ? AND is_active = ?", circleID, true).Pluck("user_id", &circleUserIDs).Error; err != nil {
		return err
	}
	var assignees []chModel.ChoreAssignees
	if err := tx.WithContext(ctx).Where("chore_id IN ?", choreIDs).Find(&assignees).Error; err != nil {
		return err
	}
	assigneesByChore := map[int][]int{}
	for _, a := range assignees {
		assigneesByChore[a.ChoreID] = append(assigneesByChore[a.ChoreID], a.UserID)
	}

	if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
		Where("project_id = ? AND circle_id = ?", project.ID, circleID).
		Update("project_id", nil).Error; err != nil {
		return err
	}
	nextVersion, err := r.nextSyncVersionRangeWithDB(ctx, tx, circleID, len(chores))
	if err != nil {
		return err
	}
	deltas := make([]visibilityDelta, 0)
	revokedChores := make([]int, 0)
	for _, chore := range chores {
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", chore.ID).
			Update("sync_version", nextVersion).Error; err != nil {
			return err
		}
		nextVersion++

		row := &chModel.Chore{IsPrivate: chore.IsPrivate, CreatedBy: chore.CreatedBy}
		previous := projectAwareChoreViewerSet(row, project, assigneesByChore[chore.ID], circleUserIDs)
		next := choreViewerSet(row, assigneesByChore[chore.ID], circleUserIDs)
		delta := visibilityDelta{choreID: chore.ID}
		for id := range previous {
			if _, ok := next[id]; !ok {
				delta.revoked = append(delta.revoked, id)
			}
		}
		for id := range next {
			if _, ok := previous[id]; !ok {
				delta.granted = append(delta.granted, id)
			}
		}
		sort.Ints(delta.revoked)
		sort.Ints(delta.granted)
		if len(delta.revoked) > 0 || len(delta.granted) > 0 {
			deltas = append(deltas, delta)
		}
		if len(delta.revoked) > 0 {
			revokedChores = append(revokedChores, chore.ID)
		}
	}
	if len(revokedChores) > 0 {
		if err := tx.WithContext(ctx).Where("chore_id IN ? AND is_sent = ?", revokedChores, false).
			Delete(&nModel.Notification{}).Error; err != nil {
			return err
		}
	}
	_, err = r.applyVisibilityDeltas(ctx, tx, circleID, deltas)
	return err
}
