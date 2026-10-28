package chore

import (
	"context"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	pModel "donetick.com/core/internal/project/model"
)

func TestNewChoresAppendToSection(t *testing.T) {
	for _, withAssignees := range []bool{false, true} {
		name := "CreateChore"
		if withAssignees {
			name = "CreateChoreWithAssignees"
		}
		t.Run(name, func(t *testing.T) {
			r, db := newTestChoreRepo(t)
			ctx := context.Background()
			project := createProject(t, db, testOwnerID, false)
			first := pModel.ProjectSection{ProjectID: project.ID, CircleID: testCircleID, Name: "Todo", Position: 1}
			second := pModel.ProjectSection{ProjectID: project.ID, CircleID: testCircleID, Name: "Doing", Position: 2}
			for _, section := range []*pModel.ProjectSection{&first, &second} {
				if err := db.Create(section).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Include an unplaced legacy card shown in the first section, plus a
			// card in another section that must not affect the first section's end.
			for _, chore := range []*chModel.Chore{
				{Name: "legacy", CircleID: testCircleID, ProjectID: &project.ID, BoardPosition: 5000},
				{Name: "other section", CircleID: testCircleID, ProjectID: &project.ID, SectionID: &second.ID, BoardPosition: 90000},
			} {
				if err := db.Create(chore).Error; err != nil {
					t.Fatal(err)
				}
			}
			create := func(sectionID *int, wantPosition int) {
				t.Helper()
				chore := &chModel.Chore{
					Name: "new", CircleID: testCircleID, CreatedBy: testOwnerID,
					ProjectID: &project.ID, SectionID: sectionID, BoardPosition: -123,
					AssignStrategy: chModel.AssignmentStrategyRandom, IsActive: true,
				}
				var err error
				if withAssignees {
					_, err = r.CreateChoreWithAssignees(ctx, chore, []int{testOwnerID}, testOwnerID)
				} else {
					_, err = r.CreateChore(ctx, chore)
				}
				if err != nil {
					t.Fatal(err)
				}
				var saved chModel.Chore
				if err := db.First(&saved, chore.ID).Error; err != nil {
					t.Fatal(err)
				}
				if saved.BoardPosition != wantPosition {
					t.Fatalf("position = %d, want %d", saved.BoardPosition, wantPosition)
				}
				if sectionID == nil && (saved.SectionID == nil || *saved.SectionID != first.ID) {
					t.Fatal("unspecified section should default to the first section")
				}
			}
			create(&first.ID, 6000)
			create(&first.ID, 7000)
			create(nil, 8000)
			create(&second.ID, 91000)
		})
	}
}
