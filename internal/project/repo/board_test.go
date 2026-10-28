package repo

import (
	"context"
	"errors"
	"testing"

	chModel "donetick.com/core/internal/chore/model"
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

// Nothing links the board tables to the project by foreign key, so deleting a
// project has to take its sections and the per-user view preferences with it;
// orphans would reappear under a recycled project id.
func TestDeleteProjectRemovesItsBoardRows(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()
	project := createTestProject(t, db, "shared", testOwnerID, false)
	kept := createTestProject(t, db, "kept", testOwnerID, false)

	if _, err := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "To Do"); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	if err := r.SetViewPreference(ctx, project.ID, testCircleID, testOwnerID, pModel.ProjectViewBoard); err != nil {
		t.Fatalf("SetViewPreference: %v", err)
	}
	if _, err := r.CreateSection(ctx, kept.ID, testCircleID, testOwnerID, "To Do"); err != nil {
		t.Fatalf("CreateSection: %v", err)
	}
	if err := r.SetViewPreference(ctx, kept.ID, testCircleID, testOwnerID, pModel.ProjectViewList); err != nil {
		t.Fatalf("SetViewPreference: %v", err)
	}

	if _, err := r.DeleteProjectWithResult(ctx, project.ID, testOwnerID, testCircleID); err != nil {
		t.Fatalf("DeleteProjectWithResult: %v", err)
	}

	var sections, prefs int64
	db.Model(&pModel.ProjectSection{}).Where("project_id = ?", project.ID).Count(&sections)
	db.Model(&pModel.ProjectViewPreference{}).Where("project_id = ?", project.ID).Count(&prefs)
	if sections != 0 || prefs != 0 {
		t.Errorf("deleted project left %d sections and %d view preferences behind", sections, prefs)
	}

	db.Model(&pModel.ProjectSection{}).Where("project_id = ?", kept.ID).Count(&sections)
	db.Model(&pModel.ProjectViewPreference{}).Where("project_id = ?", kept.ID).Count(&prefs)
	if sections != 1 || prefs != 1 {
		t.Errorf("untouched project has %d sections and %d view preferences, want 1 and 1", sections, prefs)
	}
}

// Repeatedly dropping a card at the top of a column halves the gap each time
// until there is none left, which forces the renumber path.
func TestMoveChoreRenumbersWhenNoIntegerGapIsLeft(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()
	project := createTestProject(t, db, "shared", testOwnerID, false)
	section, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Todo")

	anchor := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(ctx, project.ID, anchor.ID, section.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore anchor: %v", err)
	}
	// Each of these lands first, taking half of the remaining gap below the
	// previous head, so the positions collapse towards 0.
	heads := make([]*chModel.Chore, 0, 14)
	for i := 0; i < 14; i++ {
		chore := createTestChore(t, db, project.ID, false)
		if err := r.MoveChore(ctx, project.ID, chore.ID, section.ID, nil, testCircleID, testOwnerID); err != nil {
			t.Fatalf("MoveChore head %d: %v", i, err)
		}
		heads = append(heads, chore)
	}

	// Whatever happened in between, the column must still read newest-first with
	// strictly increasing positions and no duplicates.
	var order []positioned
	if err := db.Model(&chModel.Chore{}).Select("id, board_position AS position").
		Where("section_id = ?", section.ID).Order("board_position ASC").Scan(&order).Error; err != nil {
		t.Fatalf("reading column: %v", err)
	}
	if len(order) != len(heads)+1 {
		t.Fatalf("column has %d cards, want %d", len(order), len(heads)+1)
	}
	for i := 1; i < len(order); i++ {
		if order[i].Position <= order[i-1].Position {
			t.Fatalf("positions are not strictly increasing: %+v", order)
		}
	}
	want := make([]int, 0, len(order))
	for i := len(heads) - 1; i >= 0; i-- {
		want = append(want, heads[i].ID)
	}
	want = append(want, anchor.ID)
	for i, p := range order {
		if p.ID != want[i] {
			t.Fatalf("column order = %+v, want ids %v", order, want)
		}
	}
}

// A drag that drops a card or a column back onto itself is a no-op, not a
// "reference row is not a sibling" error.
func TestDroppingOntoItselfIsANoOp(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()
	project := createTestProject(t, db, "shared", testOwnerID, false)
	first, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Todo")
	second, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Done")

	chore := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(ctx, project.ID, chore.ID, first.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}
	placed := reloadChore(t, db, chore.ID)

	if err := r.MoveChore(ctx, project.ID, chore.ID, first.ID, &chore.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("dropping a card onto itself: %v", err)
	}
	if again := reloadChore(t, db, chore.ID); again.BoardPosition != placed.BoardPosition || *again.SectionID != first.ID {
		t.Fatalf("card moved: section %v position %d, want section %d position %d",
			again.SectionID, again.BoardPosition, first.ID, placed.BoardPosition)
	}
	// "After itself" in a section it is not in has no meaning.
	if err := r.MoveChore(ctx, project.ID, chore.ID, second.ID, &chore.ID, testCircleID, testOwnerID); !errors.Is(err, ErrChoreNotFound) {
		t.Fatalf("err = %v, want ErrChoreNotFound", err)
	}

	if err := r.MoveSection(ctx, project.ID, second.ID, &second.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("dropping a column onto itself: %v", err)
	}
	sections, _ := r.GetSections(ctx, project.ID, testCircleID)
	if len(sections) != 2 || sections[0].ID != first.ID || sections[1].ID != second.ID {
		t.Fatalf("columns reordered: %+v", sections)
	}
}

// Merging a deleted column into the fallback must keep the moved cards in
// their old order, below the cards already there.
func TestDeleteSectionKeepsTheMergedOrder(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()
	project := createTestProject(t, db, "shared", testOwnerID, false)
	todo, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Todo")
	doomed, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Doomed")

	stays := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(ctx, project.ID, stays.ID, todo.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}
	first := createTestChore(t, db, project.ID, false)
	second := createTestChore(t, db, project.ID, false)
	if err := r.MoveChore(ctx, project.ID, first.ID, doomed.ID, nil, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}
	if err := r.MoveChore(ctx, project.ID, second.ID, doomed.ID, &first.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("MoveChore: %v", err)
	}

	if err := r.DeleteSection(ctx, project.ID, doomed.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("DeleteSection: %v", err)
	}

	var order []int
	if err := db.Model(&chModel.Chore{}).Where("section_id = ?", todo.ID).
		Order("board_position ASC").Pluck("id", &order).Error; err != nil {
		t.Fatalf("reading column: %v", err)
	}
	want := []int{stays.ID, first.ID, second.ID}
	if len(order) != len(want) {
		t.Fatalf("fallback column = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("fallback column = %v, want %v", order, want)
		}
	}
}

// Chores that were never placed in a section (everything created before the
// board existed, anything created through the API, anything moved in from
// another project) are shown at the top of the first column, and clients pass
// the id of whatever card they show as afterChoreId. Dropping a card below one
// of them must work.
func TestMoveChoreCanBeDroppedAfterAnUnplacedChore(t *testing.T) {
	r, db := newTestRepo(t)
	ctx := context.Background()
	project := createTestProject(t, db, "shared", testOwnerID, false)
	first, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Todo")
	second, _ := r.CreateSection(ctx, project.ID, testCircleID, testOwnerID, "Done")

	unplaced := createTestChore(t, db, project.ID, false)
	if reloadChore(t, db, unplaced.ID).SectionID != nil {
		t.Fatal("expected the chore to start without a section")
	}
	dragged := createTestChore(t, db, project.ID, false)

	if err := r.MoveChore(ctx, project.ID, dragged.ID, first.ID, &unplaced.ID, testCircleID, testOwnerID); err != nil {
		t.Fatalf("dropping a card after an unplaced one: %v", err)
	}
	placed := reloadChore(t, db, dragged.ID)
	if placed.SectionID == nil || *placed.SectionID != first.ID {
		t.Fatalf("section = %v, want %d", placed.SectionID, first.ID)
	}
	if placed.BoardPosition <= reloadChore(t, db, unplaced.ID).BoardPosition {
		t.Fatalf("card at %d did not land after the unplaced one at %d",
			placed.BoardPosition, reloadChore(t, db, unplaced.ID).BoardPosition)
	}
	// The unplaced chore keeps its own row: it is read as a sibling, not rewritten.
	if reloadChore(t, db, unplaced.ID).SectionID != nil {
		t.Fatal("the unplaced chore should not have been adopted into the section")
	}
	// It only counts as a sibling of the first column, which is where it shows.
	if err := r.MoveChore(ctx, project.ID, dragged.ID, second.ID, &unplaced.ID, testCircleID, testOwnerID); !errors.Is(err, ErrChoreNotFound) {
		t.Fatalf("err = %v, want ErrChoreNotFound for a later column", err)
	}
}
