package model

import (
	"encoding/json"
	"sort"
	"time"
)

// ProjectVisibility is the audience of a project and of every chore inside it.
//
// It is derived from two columns instead of being stored on its own, so that the
// absence of the newer column can never widen access:
//
//	is_private = false                              -> circle  (whole circle)
//	is_private = true, shared_with_members = false  -> private (owner only)
//	is_private = true, shared_with_members = true   -> members (owner + project_members)
//
// A binary that predates shared_with_members reads a "members" project as
// private, and adding the column to an existing database defaults it to false,
// which keeps every existing project exactly as visible as before.
type ProjectVisibility string

const (
	ProjectVisibilityPrivate ProjectVisibility = "private"
	ProjectVisibilityMembers ProjectVisibility = "members"
	ProjectVisibilityCircle  ProjectVisibility = "circle"
)

// Valid reports whether v is one of the known visibilities.
func (v ProjectVisibility) Valid() bool {
	switch v {
	case ProjectVisibilityPrivate, ProjectVisibilityMembers, ProjectVisibilityCircle:
		return true
	}
	return false
}

type Project struct {
	ID          int        `json:"id" gorm:"primary_key"`
	Name        string     `json:"name" gorm:"column:name;not null"`
	Description *string    `json:"description" gorm:"column:description"`
	Color       *string    `json:"color" gorm:"column:color"`
	Icon        *string    `json:"icon" gorm:"column:icon"`
	CircleID    int        `json:"circleId" gorm:"column:circle_id;index;not null"`
	CreatedBy   int        `json:"created_by" gorm:"column:created_by;not null"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty" gorm:"column:updated_at;autoUpdateTime"`
	IsDefault   bool       `json:"isDefault" gorm:"column:is_default;default:false"`
	IsPrivate   bool       `json:"isPrivate" gorm:"column:is_private;default:false"` // Whether the project is hidden from the circle
	// SharedWithMembers opens a private project to the users listed in
	// project_members. It has no effect unless IsPrivate is also true.
	SharedWithMembers bool `json:"-" gorm:"column:shared_with_members;not null;default:false"`

	// MemberIDs holds the users a "members" project is shared with. It is not a
	// GORM association on purpose: auto-saved associations would re-insert a
	// stale member list whenever a loaded chore or project is saved again, which
	// could silently restore access that was just revoked. Fill it through
	// access.LoadMembers; until then membersLoaded is false and CanView fails
	// closed for everybody but the owner.
	MemberIDs     []int `json:"-" gorm:"-"`
	membersLoaded bool
}

// ProjectMember grants one user of the project's circle access to a project whose
// visibility is "members". Rows are ignored for every other visibility.
type ProjectMember struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	ProjectID int       `json:"projectId" gorm:"column:project_id;not null;uniqueIndex:idx_project_members_project_user,priority:1"`
	UserID    int       `json:"userId" gorm:"column:user_id;not null;uniqueIndex:idx_project_members_project_user,priority:2;index:idx_project_members_user"`
	CircleID  int       `json:"circleId" gorm:"column:circle_id;not null;index:idx_project_members_circle"`
	AddedBy   int       `json:"addedBy" gorm:"column:added_by;not null"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (ProjectMember) TableName() string { return "project_members" }

// Visibility returns the effective audience of the project.
func (p *Project) Visibility() ProjectVisibility {
	switch {
	case !p.IsPrivate:
		return ProjectVisibilityCircle
	case p.SharedWithMembers:
		return ProjectVisibilityMembers
	default:
		return ProjectVisibilityPrivate
	}
}

// SetMembers records the member list loaded for this project.
func (p *Project) SetMembers(userIDs []int) {
	members := make([]int, 0, len(userIDs))
	seen := make(map[int]struct{}, len(userIDs))
	for _, id := range userIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		members = append(members, id)
	}
	sort.Ints(members)
	p.MemberIDs = members
	p.membersLoaded = true
}

// MembersLoaded reports whether MemberIDs reflects the database.
func (p *Project) MembersLoaded() bool { return p.membersLoaded }

// IsMember reports whether userID is on the member list of a "members" project.
// It is false for every other visibility, and false while members are not loaded.
func (p *Project) IsMember(userID int) bool {
	if p.Visibility() != ProjectVisibilityMembers || !p.membersLoaded {
		return false
	}
	for _, id := range p.MemberIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// CanView reports whether the user is allowed to see the project. Circle projects
// are visible to every member of the circle, private ones only to their creator,
// and members projects to their creator plus the listed members. Circle admins
// and managers do not bypass this, same as private chores today.
func (p *Project) CanView(userID int) bool {
	return !p.IsPrivate || p.CreatedBy == userID || p.IsMember(userID)
}

// ViewerIDs returns who can see a restricted (private or members) project, owner
// first. ok is false for circle projects, whose audience is the whole circle.
func (p *Project) ViewerIDs() (ids []int, ok bool) {
	if !p.IsPrivate {
		return nil, false
	}
	ids = []int{p.CreatedBy}
	if p.Visibility() == ProjectVisibilityMembers && p.membersLoaded {
		for _, id := range p.MemberIDs {
			if id != p.CreatedBy {
				ids = append(ids, id)
			}
		}
	}
	return ids, true
}

// MarshalJSON adds the derived visibility and, once loaded, the member list.
func (p Project) MarshalJSON() ([]byte, error) {
	type alias Project
	out := struct {
		alias
		Visibility ProjectVisibility `json:"visibility"`
		MemberIDs  []int             `json:"memberIds,omitempty"`
	}{
		alias:      alias(p),
		Visibility: p.Visibility(),
	}
	if p.Visibility() == ProjectVisibilityMembers && p.membersLoaded {
		out.MemberIDs = p.MemberIDs
		if out.MemberIDs == nil {
			out.MemberIDs = []int{}
		}
	}
	return json.Marshal(out)
}

type ProjectReq struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	Icon        *string `json:"icon"`
	// IsPrivate is optional: when omitted on update the current value is kept, so
	// older clients that don't know about the field can't accidentally unset it.
	// Clients that only know isPrivate keep a "members" project shared as long as
	// they send isPrivate=true; isPrivate=false opens it to the whole circle.
	IsPrivate *bool `json:"isPrivate"`
	// Visibility takes precedence over IsPrivate when both are sent.
	Visibility *ProjectVisibility `json:"visibility"`
	// MemberIDs is only read together with visibility "members". When omitted
	// there, the current member list is kept.
	MemberIDs *[]int `json:"memberIds"`
}

// ResolveAccess turns a request into the access the project should end up with,
// starting from the current state. It never widens access on its own: a field
// the client did not send keeps its current value.
func (r *ProjectReq) ResolveAccess(current ProjectVisibility, currentMembers []int) (ProjectVisibility, []int) {
	target := current
	switch {
	case r.Visibility != nil:
		target = *r.Visibility
	case r.IsPrivate != nil:
		if !*r.IsPrivate {
			target = ProjectVisibilityCircle
		} else if current == ProjectVisibilityCircle {
			target = ProjectVisibilityPrivate
		}
	}

	if target != ProjectVisibilityMembers {
		return target, nil
	}
	if r.MemberIDs != nil {
		return target, append([]int(nil), (*r.MemberIDs)...)
	}
	if current == ProjectVisibilityMembers {
		return target, append([]int(nil), currentMembers...)
	}
	return target, []int{}
}

type UpdateProjectReq struct {
	ID int `json:"id" binding:"required"`
	ProjectReq
}
