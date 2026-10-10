package repo

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
)

// Three people in one circle: the owner of the work project, a colleague it is
// shared with, and somebody else in the same circle it is not shared with.
const (
	ownerID     = 1
	memberID    = 2
	outsiderID  = 3
	strangerID  = 99 // not in the circle at all
	sharingCirc = testCircleID
)

func seedCircle(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&cModel.Circle{ID: sharingCirc, Name: "circle", CreatedBy: ownerID, InviteCode: "invite"}).Error)
	for _, id := range []int{ownerID, memberID, outsiderID} {
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: sharingCirc, IsActive: true, Role: "member"}).Error)
	}
}

func visibilityPtr(v pModel.ProjectVisibility) *pModel.ProjectVisibility { return &v }
func idsPtr(ids ...int) *[]int                                           { return &ids }

func newSharedProject(t *testing.T, r *ProjectRepository, name string, visibility pModel.ProjectVisibility, members ...int) *pModel.Project {
	t.Helper()
	project := &pModel.Project{
		Name:              name,
		CircleID:          sharingCirc,
		CreatedBy:         ownerID,
		IsPrivate:         visibility != pModel.ProjectVisibilityCircle,
		SharedWithMembers: visibility == pModel.ProjectVisibilityMembers,
		MemberIDs:         members,
	}
	require.NoError(t, r.CreateProject(context.Background(), project))
	return project
}

func newProjectChore(t *testing.T, db *gorm.DB, projectID int, name string, createdBy int, strategy chModel.AssignmentStrategy, assignedTo *int, assignees ...int) *chModel.Chore {
	t.Helper()
	chore := &chModel.Chore{
		Name:           name,
		CircleID:       sharingCirc,
		CreatedBy:      createdBy,
		ProjectID:      &projectID,
		IsPrivate:      true,
		IsActive:       true,
		AssignStrategy: strategy,
		AssignedTo:     assignedTo,
	}
	require.NoError(t, db.Create(chore).Error)
	for _, id := range assignees {
		require.NoError(t, db.Create(&chModel.ChoreAssignees{ChoreID: chore.ID, UserID: id}).Error)
	}
	return chore
}

func addHistory(t *testing.T, db *gorm.DB, choreID, completedBy int, note string) *chModel.ChoreHistory {
	t.Helper()
	now := time.Now().UTC()
	h := &chModel.ChoreHistory{ChoreID: choreID, CompletedBy: completedBy, PerformedAt: &now, Status: chModel.ChoreHistoryStatusCompleted, Note: &note, SyncVersion: 1}
	require.NoError(t, db.Create(h).Error)
	return h
}

func projectNames(t *testing.T, r *ProjectRepository, userID int) []string {
	t.Helper()
	projects, err := r.GetCircleProjects(context.Background(), sharingCirc, userID)
	require.NoError(t, err)
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

func visibleChoreIDs(t *testing.T, r *ProjectRepository, userID int) []int {
	t.Helper()
	chores, err := r.choreRepo.GetChores(context.Background(), sharingCirc, userID, true, nil, false)
	require.NoError(t, err)
	ids := make([]int, 0, len(chores))
	for _, c := range chores {
		ids = append(ids, c.ID)
	}
	sort.Ints(ids)
	return ids
}

func userTombstones(t *testing.T, db *gorm.DB, userID int, entity syncModel.EntityType) []int {
	t.Helper()
	var rows []syncModel.Tombstone
	require.NoError(t, db.Where("user_id = ? AND entity_type = ?", userID, entity).Order("entity_id").Find(&rows).Error)
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.EntityID)
	}
	return ids
}

func TestProjectAudienceForOwnerMemberAndOutsider(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()

	private := newSharedProject(t, r, "Privat", pModel.ProjectVisibilityPrivate)
	work := newSharedProject(t, r, "Installation A", pModel.ProjectVisibilityMembers, memberID)
	household := newSharedProject(t, r, "Haushalt", pModel.ProjectVisibilityCircle)

	privateChore := newProjectChore(t, db, private.ID, "private", ownerID, chModel.AssignmentStrategyRandom, intp(ownerID), ownerID)
	workChore := newProjectChore(t, db, work.ID, "work", ownerID, chModel.AssignmentStrategyRandom, intp(ownerID), ownerID)
	householdChore := newProjectChore(t, db, household.ID, "household", ownerID, chModel.AssignmentStrategyRandom, intp(ownerID), ownerID)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", householdChore.ID).Update("is_private", false).Error)

	require.Equal(t, []string{"Haushalt", "Installation A", "Privat"}, projectNames(t, r, ownerID))
	require.Equal(t, []string{"Haushalt", "Installation A"}, projectNames(t, r, memberID))
	require.Equal(t, []string{"Haushalt"}, projectNames(t, r, outsiderID))

	require.Equal(t, []int{privateChore.ID, workChore.ID, householdChore.ID}, visibleChoreIDs(t, r, ownerID))
	require.Equal(t, []int{workChore.ID, householdChore.ID}, visibleChoreIDs(t, r, memberID))
	require.Equal(t, []int{householdChore.ID}, visibleChoreIDs(t, r, outsiderID))

	// Direct access by ID goes through the same predicate.
	_, err := r.choreRepo.GetChore(ctx, workChore.ID, outsiderID, sharingCirc)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = r.choreRepo.GetChore(ctx, privateChore.ID, memberID, sharingCirc)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	loaded, err := r.choreRepo.GetChore(ctx, workChore.ID, memberID, sharingCirc)
	require.NoError(t, err)
	require.True(t, loaded.CanView(memberID, nil), "member must pass the handler-level check too")
	require.False(t, loaded.CanView(outsiderID, nil))
	_, err = r.choreRepo.GetChoreDetailByID(ctx, workChore.ID, sharingCirc, outsiderID)
	require.Error(t, err)

	// History listings follow the chore.
	addHistory(t, db, workChore.ID, memberID, "done on site")
	hist, err := r.choreRepo.GetChoresHistoryByUserID(ctx, outsiderID, sharingCirc, 30, true)
	require.NoError(t, err)
	require.Empty(t, hist)
	hist, err = r.choreRepo.GetChoresHistoryByUserID(ctx, memberID, sharingCirc, 30, true)
	require.NoError(t, err)
	require.Len(t, hist, 1)

	// The member list is visible on the project and serialized for its audience.
	projects, err := r.GetCircleProjects(ctx, sharingCirc, memberID)
	require.NoError(t, err)
	for _, p := range projects {
		if p.ID == work.ID {
			require.Equal(t, pModel.ProjectVisibilityMembers, p.Visibility())
			require.Equal(t, []int{memberID}, p.MemberIDs)
		}
	}
}

func TestOnlyActiveCircleMembersCanBeAdded(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	require.NoError(t, db.Create(&cModel.UserCircle{UserID: 7, CircleID: sharingCirc, IsActive: false}).Error)

	err := r.CreateProject(context.Background(), &pModel.Project{Name: "x", CircleID: sharingCirc, CreatedBy: ownerID, IsPrivate: true, SharedWithMembers: true, MemberIDs: []int{strangerID}})
	require.ErrorIs(t, err, ErrMemberNotInCircle)
	err = r.CreateProject(context.Background(), &pModel.Project{Name: "y", CircleID: sharingCirc, CreatedBy: ownerID, IsPrivate: true, SharedWithMembers: true, MemberIDs: []int{7}})
	require.ErrorIs(t, err, ErrMemberNotInCircle, "pending join requests are not members yet")

	// The owner is implied and silently dropped from the list; duplicates collapse.
	p := newSharedProject(t, r, "z", pModel.ProjectVisibilityMembers, ownerID, memberID, memberID)
	reloaded, err := r.GetProjectByID(context.Background(), p.ID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, []int{memberID}, reloaded.MemberIDs)
}

func TestClientsThatOnlyKnowIsPrivateKeepTheShare(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()
	work := newSharedProject(t, r, "work", pModel.ProjectVisibilityMembers, memberID)

	// An old client renames the project without sending any privacy field.
	_, err := r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "renamed"}, &pModel.ProjectReq{}, ownerID, sharingCirc)
	require.NoError(t, err)
	// A client that knows isPrivate (upstream UI) saves the checkbox as it shows it.
	yes := true
	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "renamed"}, &pModel.ProjectReq{IsPrivate: &yes}, ownerID, sharingCirc)
	require.NoError(t, err)

	reloaded, err := r.GetProjectByID(ctx, work.ID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, pModel.ProjectVisibilityMembers, reloaded.Visibility())
	require.Equal(t, []int{memberID}, reloaded.MemberIDs)

	// Unticking "private" opens it to the circle and drops the member rows.
	no := false
	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "renamed"}, &pModel.ProjectReq{IsPrivate: &no}, ownerID, sharingCirc)
	require.NoError(t, err)
	reloaded, err = r.GetProjectByID(ctx, work.ID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, pModel.ProjectVisibilityCircle, reloaded.Visibility())
	var rows int64
	require.NoError(t, db.Model(&pModel.ProjectMember{}).Where("project_id = ?", work.ID).Count(&rows).Error)
	require.Zero(t, rows)
}

func TestOnlyTheOwnerManagesAccess(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()
	work := newSharedProject(t, r, "work", pModel.ProjectVisibilityMembers, memberID)

	_, err := r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "work"},
		&pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr(memberID, outsiderID)}, memberID, sharingCirc)
	require.EqualError(t, err, "user does not have permission to update this project")

	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "work"},
		&pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityCircle)}, outsiderID, sharingCirc)
	require.ErrorIs(t, err, ErrProjectNotFound, "outsiders must not learn the project exists")

	_, err = r.PreviewProjectAccess(ctx, work.ID, &pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityCircle)}, outsiderID, sharingCirc)
	require.ErrorIs(t, err, ErrProjectNotFound)
	require.ErrorIs(t, r.DeleteProject(ctx, work.ID, outsiderID, sharingCirc), ErrProjectNotFound)
	require.EqualError(t, r.DeleteProject(ctx, work.ID, memberID, sharingCirc), "user does not have permission to delete this project")

	bad := pModel.ProjectVisibility("everyone")
	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "work"}, &pModel.ProjectReq{Visibility: &bad}, ownerID, sharingCirc)
	require.ErrorIs(t, err, ErrInvalidVisibility)
}

func TestRevokingAMemberKeepsTasksAndHistory(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()
	work := newSharedProject(t, r, "Installation A", pModel.ProjectVisibilityMembers, memberID)

	onSite := newProjectChore(t, db, work.ID, "on site", ownerID, chModel.AssignmentStrategyRoundRobin, intp(memberID), ownerID, memberID)
	memberOnly := newProjectChore(t, db, work.ID, "member only", memberID, chModel.AssignmentStrategyRandom, intp(memberID), memberID)
	unassigned := newProjectChore(t, db, work.ID, "anyone", ownerID, chModel.AssignmentStrategyNoAssignee, nil)
	h1 := addHistory(t, db, onSite.ID, memberID, "installed")
	h2 := addHistory(t, db, memberOnly.ID, memberID, "documented")
	require.NoError(t, db.Create(&nModel.Notification{ChoreID: onSite.ID, CircleID: sharingCirc, UserID: memberID, Text: "reminder"}).Error)

	// Preview first: it must describe the effect without writing anything.
	preview, err := r.PreviewProjectAccess(ctx, work.ID, &pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr()}, ownerID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, []int{memberID}, preview.RevokedUsers)
	changed := map[int]*chRepo.ChoreAccessChange{}
	for _, c := range preview.ChangedChores() {
		changed[c.ChoreID] = c
	}
	require.Equal(t, []int{memberID}, changed[onSite.ID].RemovedAssignees)
	require.Equal(t, ownerID, *changed[onSite.ID].AssignedToAfter)
	require.Equal(t, []int{memberID}, changed[memberOnly.ID].RemovedAssignees)
	require.Equal(t, []int{ownerID}, changed[memberOnly.ID].AddedAssignees, "a chore must not silently turn into Anyone")
	require.Equal(t, []int{memberID}, changed[unassigned.ID].RevokedUsers)
	var stillMember int64
	require.NoError(t, db.Model(&pModel.ProjectMember{}).Where("project_id = ? AND user_id = ?", work.ID, memberID).Count(&stillMember).Error)
	require.EqualValues(t, 1, stillMember, "preview must not change anything")

	// Project done: remove the colleague, keep the project shared-ready.
	result, err := r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "Installation A"},
		&pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr()}, ownerID, sharingCirc)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Revocations[onSite.ID], memberID)

	require.Equal(t, []string{}, nonNil(projectNames(t, r, memberID)))
	require.Empty(t, visibleChoreIDs(t, r, memberID))
	require.Equal(t, []int{onSite.ID, memberOnly.ID, unassigned.ID}, visibleChoreIDs(t, r, ownerID), "the owner keeps every task, including the ones the colleague created")
	for _, id := range []int{onSite.ID, memberOnly.ID, unassigned.ID} {
		_, err := r.choreRepo.GetChore(ctx, id, memberID, sharingCirc)
		require.Error(t, err)
	}

	// Tasks and their history are still there, only no longer shared.
	var histories []chModel.ChoreHistory
	require.NoError(t, db.Where("id IN ?", []int{h1.ID, h2.ID}).Find(&histories).Error)
	require.Len(t, histories, 2)
	for _, h := range histories {
		require.Equal(t, memberID, h.CompletedBy)
	}
	ownerHistory, err := r.choreRepo.GetChoreHistory(ctx, onSite.ID)
	require.NoError(t, err)
	require.Len(t, ownerHistory, 1)

	// Assignments now point at people who can see the task.
	var reloaded chModel.Chore
	require.NoError(t, db.First(&reloaded, onSite.ID).Error)
	require.Equal(t, ownerID, *reloaded.AssignedTo)
	assertAssigneeIDs(t, db, onSite.ID, []int{ownerID})
	assertAssigneeIDs(t, db, memberOnly.ID, []int{ownerID})

	// Synced clients of the colleague drop the tasks and their history.
	require.Equal(t, []int{onSite.ID, memberOnly.ID, unassigned.ID}, userTombstones(t, db, memberID, syncModel.EntityTypeChore))
	require.ElementsMatch(t, []int{h1.ID, h2.ID}, userTombstones(t, db, memberID, syncModel.EntityTypeChoreHistory))
	tombs, err := r.choreRepo.GetTombstonesSince(ctx, sharingCirc, syncModel.EntityTypeChore, outsiderID, 0, 0)
	require.NoError(t, err)
	require.Empty(t, tombs, "nobody else is affected")

	// Pending reminders that could reach the colleague are gone.
	var pending int64
	require.NoError(t, db.Model(&nModel.Notification{}).Where("user_id = ? AND is_sent = ?", memberID, false).Count(&pending).Error)
	require.Zero(t, pending)

	// Sharing again restores visibility (not the old assignments) and cleans up the
	// stale tombstones so a client syncing both steps at once keeps the tasks.
	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: work.ID, Name: "Installation A"},
		&pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr(memberID)}, ownerID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, []int{onSite.ID, memberOnly.ID, unassigned.ID}, visibleChoreIDs(t, r, memberID))
	require.Empty(t, userTombstones(t, db, memberID, syncModel.EntityTypeChore))
	require.Empty(t, userTombstones(t, db, memberID, syncModel.EntityTypeChoreHistory))
	var rehist chModel.ChoreHistory
	require.NoError(t, db.First(&rehist, h1.ID).Error)
	require.Greater(t, rehist.SyncVersion, int64(1), "history must be re-sent to the regained member")
}

func TestSwitchingCircleProjectToMembersOnlyRevokesOthers(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()
	project := newSharedProject(t, r, "was shared", pModel.ProjectVisibilityCircle)
	chore := newProjectChore(t, db, project.ID, "c", ownerID, chModel.AssignmentStrategyRandom, intp(outsiderID), ownerID, outsiderID)
	require.NoError(t, db.Model(&chModel.Chore{}).Where("id = ?", chore.ID).Update("is_private", false).Error)

	plan, err := r.PreviewProjectAccess(ctx, project.ID, &pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr(memberID)}, ownerID, sharingCirc)
	require.NoError(t, err)
	require.Equal(t, []int{outsiderID}, plan.RevokedUsers)
	require.Empty(t, plan.GrantedUsers)

	_, err = r.UpdateProjectWithAccess(ctx, &pModel.Project{ID: project.ID, Name: "was shared"},
		&pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr(memberID)}, ownerID, sharingCirc)
	require.NoError(t, err)

	require.Empty(t, visibleChoreIDs(t, r, outsiderID))
	require.Equal(t, []int{chore.ID}, visibleChoreIDs(t, r, memberID))
	var reloaded chModel.Chore
	require.NoError(t, db.First(&reloaded, chore.ID).Error)
	require.True(t, reloaded.IsPrivate, "chores inherit the restriction so an older binary fails closed")
	require.Equal(t, ownerID, *reloaded.AssignedTo)
	assertAssigneeIDs(t, db, chore.ID, []int{ownerID})
}

func TestDeletingASharedProjectRevokesMembersButKeepsTasks(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	ctx := context.Background()
	work := newSharedProject(t, r, "work", pModel.ProjectVisibilityMembers, memberID)
	chore := newProjectChore(t, db, work.ID, "c", ownerID, chModel.AssignmentStrategyRandom, intp(ownerID), ownerID)

	require.NoError(t, r.DeleteProject(ctx, work.ID, ownerID, sharingCirc))

	var rows int64
	require.NoError(t, db.Model(&pModel.ProjectMember{}).Where("project_id = ?", work.ID).Count(&rows).Error)
	require.Zero(t, rows)
	require.Equal(t, []int{chore.ID}, visibleChoreIDs(t, r, ownerID))
	require.Empty(t, visibleChoreIDs(t, r, memberID))
	require.Equal(t, []int{chore.ID}, userTombstones(t, db, memberID, syncModel.EntityTypeChore))
}

func TestStaleMemberRowsNeverOpenAProject(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	private := newSharedProject(t, r, "mine", pModel.ProjectVisibilityPrivate)
	chore := newProjectChore(t, db, private.ID, "c", ownerID, chModel.AssignmentStrategyRandom, intp(ownerID), ownerID)
	// A row left behind (e.g. by an older binary that ignores the table) must not
	// grant anything while the project is not marked as shared.
	require.NoError(t, db.Create(&pModel.ProjectMember{ProjectID: private.ID, UserID: memberID, CircleID: sharingCirc, AddedBy: ownerID}).Error)

	require.Equal(t, []string{}, nonNil(projectNames(t, r, memberID)))
	require.Empty(t, visibleChoreIDs(t, r, memberID))
	_, err := r.choreRepo.GetChore(context.Background(), chore.ID, memberID, sharingCirc)
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func assertAssigneeIDs(t *testing.T, db *gorm.DB, choreID int, want []int) {
	t.Helper()
	var ids []int
	require.NoError(t, db.Model(&chModel.ChoreAssignees{}).Where("chore_id = ?", choreID).Order("user_id").Pluck("user_id", &ids).Error)
	require.Equal(t, want, ids)
}

func intp(v int) *int { return &v }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func TestAccessPreviewSerializesForClients(t *testing.T) {
	r, db := newTestRepo(t)
	seedCircle(t, db)
	work := newSharedProject(t, r, "work", pModel.ProjectVisibilityCircle)
	plan, err := r.PreviewProjectAccess(context.Background(), work.ID, &pModel.ProjectReq{Visibility: visibilityPtr(pModel.ProjectVisibilityMembers), MemberIDs: idsPtr(memberID)}, ownerID, sharingCirc)
	require.NoError(t, err)
	raw, err := json.Marshal(plan)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"before":{"visibility":"circle","memberIds":[]}`)
	require.Contains(t, string(raw), `"after":{"visibility":"members","memberIds":[2]}`)
}
