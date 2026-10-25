package repo

import (
	"context"
	"errors"
	"testing"

	pModel "donetick.com/core/internal/project/model"
)

func TestSectionLifecycle(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)

	first, err := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "To Do")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	second, err := r.CreateSection(context.Background(), project.ID, testCircleID, testOtherID, "Done")
	if err != nil {
		t.Fatalf("CreateSection by non-owner viewer: %v", err)
	}
	if second.Position <= first.Position {
		t.Fatalf("expected second section to sort after the first, got %d <= %d", second.Position, first.Position)
	}

	sections, err := r.GetSections(context.Background(), project.ID, testCircleID)
	if err != nil {
		t.Fatalf("GetSections: %v", err)
	}
	if len(sections) != 2 || sections[0].ID != first.ID || sections[1].ID != second.ID {
		t.Fatalf("unexpected section order: %+v", sections)
	}

	if err := r.RenameSection(context.Background(), project.ID, first.ID, testCircleID, testOtherID, "Todo"); err != nil {
		t.Fatalf("RenameSection: %v", err)
	}
	sections, _ = r.GetSections(context.Background(), project.ID, testCircleID)
	if sections[0].Name != "Todo" {
		t.Fatalf("expected renamed section, got %q", sections[0].Name)
	}

	if err := r.MoveSection(context.Background(), project.ID, second.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	sections, _ = r.GetSections(context.Background(), project.ID, testCircleID)
	if sections[0].ID != second.ID || sections[1].ID != first.ID {
		t.Fatalf("expected reordered sections, got %+v", sections)
	}
}

func TestMoveSectionDoesNotTouchOtherSectionsInTheCommonCase(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	a, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "A")
	b, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "B")
	c, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "C")

	// Move C to sit right after A, between A and B. B must keep its exact
	// position: a drag that only moves one card should not rewrite its siblings.
	if err := r.MoveSection(context.Background(), project.ID, c.ID, &a.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	sections, _ := r.GetSections(context.Background(), project.ID, testCircleID)
	if len(sections) != 3 || sections[0].ID != a.ID || sections[1].ID != c.ID || sections[2].ID != b.ID {
		t.Fatalf("expected order A, C, B, got %+v", sections)
	}
	for _, s := range sections {
		if s.ID == b.ID && s.Position != b.Position {
			t.Fatalf("expected B's position to stay %d, got %d", b.Position, s.Position)
		}
	}
}

func TestMoveSectionRenumbersWhenNoIntegerGapIsLeft(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	a, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "A")
	b, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "B")
	c, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "C")

	// Force A and B to adjacent integer positions so there is no room between
	// them; MoveSection must renumber and retry rather than fail or collide.
	if err := db.Model(&pModel.ProjectSection{}).Where("id = ?", a.ID).Update("position", 10).Error; err != nil {
		t.Fatalf("failed to force position: %v", err)
	}
	if err := db.Model(&pModel.ProjectSection{}).Where("id = ?", b.ID).Update("position", 11).Error; err != nil {
		t.Fatalf("failed to force position: %v", err)
	}

	if err := r.MoveSection(context.Background(), project.ID, c.ID, &a.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	sections, _ := r.GetSections(context.Background(), project.ID, testCircleID)
	if len(sections) != 3 || sections[0].ID != a.ID || sections[1].ID != c.ID || sections[2].ID != b.ID {
		t.Fatalf("expected order A, C, B after forced renumber, got %+v", sections)
	}
}

func TestDeleteSectionRejectsLastSection(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	only, err := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "Only")
	if err != nil {
		t.Fatalf("CreateSection: %v", err)
	}

	if err := r.DeleteSection(context.Background(), project.ID, only.ID, testCircleID, testOwnerID); !errors.Is(err, ErrLastSection) {
		t.Fatalf("expected ErrLastSection, got %v", err)
	}
}

func TestDeleteSectionMovesChoresToFallback(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	todo, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "Todo")
	done, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "Done")

	chore := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(context.Background(), project.ID, chore.ID, done.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}

	if err := r.DeleteSection(context.Background(), project.ID, done.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}

	reloaded := reloadChore(t, db, chore.ID)
	if reloaded.SectionID == nil || *reloaded.SectionID != todo.ID {
		t.Fatalf("expected chore to fall back to remaining section %d, got %+v", todo.ID, reloaded.SectionID)
	}
}

func TestMoveChoreRejectsChoreFromAnotherProject(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	other := createTestProject(t, db, "other", testOwnerID, false)
	section, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "Todo")

	notMine := createTestChore(t, db, other.ID, false)

	if err := r.MoveChore(context.Background(), project.ID, notMine.ID, section.ID, nil, testCircleID, testOwnerID); !errors.Is(err, ErrChoreNotFound) {
		t.Fatalf("expected ErrChoreNotFound, got %v", err)
	}
	reloadedNotMine := reloadChore(t, db, notMine.ID)
	if reloadedNotMine.SectionID != nil {
		t.Fatalf("expected chore of another project to be left untouched, got %+v", reloadedNotMine.SectionID)
	}
}

func TestMoveChoreDoesNotTouchOtherCardsInTheCommonCase(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)
	section, _ := r.CreateSection(context.Background(), project.ID, testCircleID, testOwnerID, "Todo")

	a := createTestChore(t, db, project.ID, false)
	b := createTestChore(t, db, project.ID, false)
	c := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(context.Background(), project.ID, a.ID, section.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore a: %v", err)
	}
	if err := r.MoveChore(context.Background(), project.ID, b.ID, section.ID, &a.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore b: %v", err)
	}
	if err := r.MoveChore(context.Background(), project.ID, c.ID, section.ID, &b.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore c: %v", err)
	}
	bBefore := reloadChore(t, db, b.ID)

	// Move C to sit right after A, between A and B. B must keep its exact
	// position: moving one card should not rewrite its siblings.
	if err := r.MoveChore(context.Background(), project.ID, c.ID, section.ID, &a.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}

	bAfter := reloadChore(t, db, b.ID)
	if bAfter.BoardPosition != bBefore.BoardPosition {
		t.Fatalf("expected B's position to stay %d, got %d", bBefore.BoardPosition, bAfter.BoardPosition)
	}
	cAfter := reloadChore(t, db, c.ID)
	if cAfter.BoardPosition <= reloadChore(t, db, a.ID).BoardPosition || cAfter.BoardPosition >= bAfter.BoardPosition {
		t.Fatalf("expected C between A and B, got A=%d C=%d B=%d", reloadChore(t, db, a.ID).BoardPosition, cAfter.BoardPosition, bAfter.BoardPosition)
	}
}

func TestViewPreferenceDefaultsToBoardAndIsPrivatePerUser(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "shared", testOwnerID, false)

	view, err := r.GetViewPreference(context.Background(), project.ID, testCircleID, testOtherID)
	if err != nil {
		t.Fatalf("GetViewPreference: %v", err)
	}
	if view != pModel.ProjectViewBoard {
		t.Fatalf("expected default view %q, got %q", pModel.ProjectViewBoard, view)
	}

	if err := r.SetViewPreference(context.Background(), project.ID, testCircleID, testOtherID, pModel.ProjectViewList); err != nil {
		t.Fatalf("SetViewPreference: %v", err)
	}

	otherView, err := r.GetViewPreference(context.Background(), project.ID, testCircleID, testOtherID)
	if err != nil {
		t.Fatalf("GetViewPreference: %v", err)
	}
	if otherView != pModel.ProjectViewList {
		t.Fatalf("expected saved view %q, got %q", pModel.ProjectViewList, otherView)
	}

	ownerView, err := r.GetViewPreference(context.Background(), project.ID, testCircleID, testOwnerID)
	if err != nil {
		t.Fatalf("GetViewPreference: %v", err)
	}
	if ownerView != pModel.ProjectViewBoard {
		t.Fatalf("expected owner's own preference to stay default, got %q", ownerView)
	}
}

func TestBoardOperationsRejectNonViewers(t *testing.T) {
	r, db := newTestRepo(t)
	project := createTestProject(t, db, "private", testOwnerID, true)

	if _, err := r.CreateSection(context.Background(), project.ID, testCircleID, testOtherID, "Todo"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound for non-viewer, got %v", err)
	}
}
