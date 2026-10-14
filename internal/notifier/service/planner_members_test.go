package service

import (
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	nModel "donetick.com/core/internal/notifier/model"
	pModel "donetick.com/core/internal/project/model"
)

func sharedProject(ownerID int, members ...int) *pModel.Project {
	p := &pModel.Project{ID: 40, CreatedBy: ownerID, IsPrivate: true, SharedWithMembers: true}
	p.SetMembers(members)
	return p
}

func TestAnyoneChoreInSharedProjectRemindsOnlyItsAudience(t *testing.T) {
	circleMembers := []*cModel.UserCircleDetail{
		member(1, true, nModel.NotificationPlatformPushover, "owner"),
		member(2, true, nModel.NotificationPlatformTelegram, "colleague"),
		member(3, true, nModel.NotificationPlatformPushover, "outsider"),
	}
	chore := choreDueTomorrow()
	chore.CreatedBy = 1
	chore.IsPrivate = true
	project := sharedProject(1, 2)
	chore.ProjectID = &project.ID
	chore.Project = project

	got := anyoneChoreRecipients(chore, circleMembers)
	recipients := map[int]bool{}
	for _, r := range got {
		recipients[r.UserID] = true
	}
	if len(got) != 2 || !recipients[1] || !recipients[2] {
		t.Errorf("got recipients %v, want owner (1) and colleague (2)", recipients)
	}
}

func TestSharedProjectWithUnloadedMembersFailsClosed(t *testing.T) {
	circleMembers := []*cModel.UserCircleDetail{
		member(1, true, nModel.NotificationPlatformPushover, "owner"),
		member(2, true, nModel.NotificationPlatformTelegram, "colleague"),
	}
	chore := choreDueTomorrow()
	chore.IsPrivate = true
	project := &pModel.Project{ID: 40, CreatedBy: 1, IsPrivate: true, SharedWithMembers: true}
	chore.ProjectID = &project.ID
	chore.Project = project // members never loaded

	got := anyoneChoreRecipients(chore, circleMembers)
	if len(got) != 1 || got[0].UserID != 1 {
		t.Errorf("without a loaded member list only the owner may be reminded, got %d recipients", len(got))
	}
}

func TestRestrictedProjectChoresAreNeverPostedToTheCircleGroup(t *testing.T) {
	chore := choreDueTomorrow()
	chore.NotificationMetadataV2.CircleGroup = true
	project := sharedProject(1, 2)
	chore.ProjectID = &project.ID
	chore.Project = project
	if !inRestrictedProject(chore) {
		t.Fatal("a members project must count as restricted")
	}

	chore.Project = &pModel.Project{ID: 40, CreatedBy: 1}
	if inRestrictedProject(chore) {
		t.Fatal("a circle project is not restricted")
	}

	chore.Project = nil
	if !inRestrictedProject(chore) {
		t.Fatal("an unloaded project must fail closed")
	}
}
