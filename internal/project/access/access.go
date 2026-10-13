// Package access loads and checks who may see a project. It sits between the
// project and chore repositories so both can use it without an import cycle.
package access

import (
	"context"

	pModel "donetick.com/core/internal/project/model"
	"gorm.io/gorm"
)

// LoadMembers fills MemberIDs of every "members" project in projects with one
// query. Projects with any other visibility are marked as loaded with no members,
// since their member rows (if any) carry no meaning. nil entries are skipped.
func LoadMembers(ctx context.Context, db *gorm.DB, projects ...*pModel.Project) error {
	ids := make([]int, 0, len(projects))
	for _, p := range projects {
		if p == nil {
			continue
		}
		if p.Visibility() == pModel.ProjectVisibilityMembers {
			ids = append(ids, p.ID)
		} else {
			p.SetMembers(nil)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var rows []pModel.ProjectMember
	if err := db.WithContext(ctx).Where("project_id IN ?", ids).Order("user_id asc").Find(&rows).Error; err != nil {
		return err
	}
	byProject := make(map[int][]int, len(ids))
	for _, row := range rows {
		byProject[row.ProjectID] = append(byProject[row.ProjectID], row.UserID)
	}
	for _, p := range projects {
		if p != nil && p.Visibility() == pModel.ProjectVisibilityMembers {
			p.SetMembers(byProject[p.ID])
		}
	}
	return nil
}

// ViewerSet returns who can see a chore that lives in project. ok is false when
// the project does not restrict visibility (or there is no project), in which case
// the chore's own privacy rule applies.
func ViewerSet(project *pModel.Project) (viewers map[int]struct{}, ok bool) {
	if project == nil {
		return nil, false
	}
	ids, restricted := project.ViewerIDs()
	if !restricted {
		return nil, false
	}
	viewers = make(map[int]struct{}, len(ids))
	for _, id := range ids {
		viewers[id] = struct{}{}
	}
	return viewers, true
}
