package chore

import (
	"context"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	xpModel "donetick.com/core/internal/points"
	xpRepo "donetick.com/core/internal/points/repo"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
	"github.com/stretchr/testify/require"
)

// Creation must enforce category and board ownership in the same transaction,
// including the entry point that also writes assignees.
func TestXPBoardCreationIsAtomic(t *testing.T) {
	for _, withAssignees := range []bool{false, true} {
		entry := "CreateChore"
		if withAssignees {
			entry = "CreateChoreWithAssignees"
		}
		t.Run(entry, func(t *testing.T) {
			for _, scenario := range []string{"categorized", "uncategorized", "foreign category", "archived category", "missing category", "foreign section", "missing section"} {
				t.Run(scenario, func(t *testing.T) {
					r, db := newTestChoreRepo(t)
					project := createProject(t, db, testOwnerID, false)
					section := pModel.ProjectSection{ProjectID: project.ID, CircleID: testCircleID, Name: "Todo", Position: 1}
					require.NoError(t, db.Create(&section).Error)
					category := xpModel.XPCategory{CircleID: testCircleID, Name: "Home", NameKey: "home"}
					if scenario == "foreign category" {
						category.CircleID = testCircleID + 1
					}
					category.Archived = scenario == "archived category"
					require.NoError(t, db.Create(&category).Error)
					seed := chModel.Chore{Name: "Existing", CircleID: testCircleID, ProjectID: &project.ID, SectionID: &section.ID, BoardPosition: 5000}
					require.NoError(t, db.Create(&seed).Error)
					cursor := syncModel.SyncCursor{CircleID: testCircleID, EntityType: syncModel.EntityTypeChore, MaxVersion: 9}
					require.NoError(t, db.Create(&cursor).Error)
					points, categoryID, sectionID := 30, category.ID, section.ID
					if scenario == "uncategorized" {
						categoryID = 0
					}
					if scenario == "missing category" {
						categoryID = category.ID + 1000
					}
					if scenario == "missing section" {
						sectionID = section.ID + 1000
					}
					if scenario == "foreign section" {
						otherProject := createProject(t, db, testOwnerID, false)
						otherSection := pModel.ProjectSection{ProjectID: otherProject.ID, CircleID: testCircleID, Name: "Other", Position: 1}
						require.NoError(t, db.Create(&otherSection).Error)
						sectionID = otherSection.ID
					}
					chore := &chModel.Chore{
						Name: "New XP task", CircleID: testCircleID, CreatedBy: testOwnerID, IsActive: true,
						ProjectID: &project.ID, SectionID: &sectionID, BoardPosition: -99,
						Points: &points, XPCategoryID: &categoryID, AssignStrategy: chModel.AssignmentStrategyRandom,
					}
					var err error
					if withAssignees {
						_, err = r.CreateChoreWithAssignees(context.Background(), chore, []int{testOwnerID}, testOwnerID)
					} else {
						_, err = r.CreateChore(context.Background(), chore)
					}
					valid := scenario == "categorized" || scenario == "uncategorized"
					var choreCount, assigneeCount int64
					require.NoError(t, db.Model(&chModel.Chore{}).Count(&choreCount).Error)
					require.NoError(t, db.Model(&chModel.ChoreAssignees{}).Count(&assigneeCount).Error)
					require.NoError(t, db.First(&cursor, cursor.ID).Error)
					if !valid {
						if scenario == "foreign section" || scenario == "missing section" {
							require.ErrorIs(t, err, ErrSectionOutsideProject)
						} else {
							require.ErrorIs(t, err, xpRepo.ErrInvalidCategory)
						}
						require.EqualValues(t, 1, choreCount)
						require.Zero(t, assigneeCount)
						require.EqualValues(t, 9, cursor.MaxVersion, "rejected creates must not advance synchronization")
						return
					}
					require.NoError(t, err)
					require.EqualValues(t, 2, choreCount)
					wantAssignees := int64(0)
					if withAssignees {
						wantAssignees = 1
					}
					require.Equal(t, wantAssignees, assigneeCount)
					saved := reloadXPChore(t, db, chore.ID)
					require.Equal(t, section.ID, *saved.SectionID)
					require.Equal(t, 6000, saved.BoardPosition)
					require.Equal(t, 30, *saved.Points)
					require.EqualValues(t, 10, saved.SyncVersion)
					require.EqualValues(t, 10, cursor.MaxVersion)
					if categoryID == 0 {
						require.Nil(t, saved.XPCategoryID)
					} else {
						require.Equal(t, category.ID, *saved.XPCategoryID)
					}
				})
			}
		})
	}
}

func TestXPBoardEditsRetainPlacementAndProjectMovesRetainXP(t *testing.T) {
	r, db, chore, categoryID := xpChoreFixture(t)
	project := createProject(t, db, testOwnerID, false)
	section := pModel.ProjectSection{ProjectID: project.ID, CircleID: testCircleID, Name: "Doing", Position: 1}
	require.NoError(t, db.Create(&section).Error)
	chore.ProjectID, chore.SectionID, chore.BoardPosition = &project.ID, &section.ID, 7000
	require.NoError(t, db.Save(chore).Error)
	points := 45
	chore.Points, chore.SectionID, chore.BoardPosition = &points, nil, -1
	require.NoError(t, r.UpdateChoreVisibilityForUser(context.Background(), chore, []int{1, 2}, testOwnerID))
	saved := reloadXPChore(t, db, chore.ID)
	require.Equal(t, section.ID, *saved.SectionID)
	require.Equal(t, 7000, saved.BoardPosition)
	require.Equal(t, categoryID, *saved.XPCategoryID)
	require.Equal(t, 45, *saved.Points)

	destination := createProject(t, db, testOwnerID, false)
	saved.ProjectID = &destination.ID
	require.NoError(t, r.UpdateChoreVisibilityForUser(context.Background(), saved, []int{1, 2}, testOwnerID))
	saved = reloadXPChore(t, db, chore.ID)
	require.Nil(t, saved.SectionID)
	require.Zero(t, saved.BoardPosition)
	require.Equal(t, categoryID, *saved.XPCategoryID)
	require.Equal(t, 45, *saved.Points)

	zero := 0
	saved.XPCategoryID = &zero
	require.NoError(t, r.UpdateChoreVisibilityForUser(context.Background(), saved, []int{1, 2}, testOwnerID))
	require.Nil(t, reloadXPChore(t, db, chore.ID).XPCategoryID)
}
