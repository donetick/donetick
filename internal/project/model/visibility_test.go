package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestVisibilityIsDerivedSoOldRowsKeepTheirMeaning(t *testing.T) {
	cases := []struct {
		isPrivate, shared bool
		want              ProjectVisibility
	}{
		{false, false, ProjectVisibilityCircle},
		{false, true, ProjectVisibilityCircle}, // shared flag means nothing without is_private
		{true, false, ProjectVisibilityPrivate},
		{true, true, ProjectVisibilityMembers},
	}
	for _, c := range cases {
		p := &Project{IsPrivate: c.isPrivate, SharedWithMembers: c.shared}
		if got := p.Visibility(); got != c.want {
			t.Errorf("is_private=%v shared=%v: got %q, want %q", c.isPrivate, c.shared, got, c.want)
		}
	}
}

func TestMembersOnlyCountOnceLoaded(t *testing.T) {
	p := &Project{CreatedBy: 1, IsPrivate: true, SharedWithMembers: true, MemberIDs: []int{2}}
	if p.CanView(2) {
		t.Fatal("an unloaded member list must not grant access")
	}
	p.SetMembers([]int{2, 2, 3})
	if !p.CanView(2) || !p.CanView(3) || p.CanView(4) || !p.CanView(1) {
		t.Fatal("owner and listed members only")
	}
	ids, restricted := p.ViewerIDs()
	if !restricted || !reflect.DeepEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("viewer ids = %v, %v", ids, restricted)
	}
}

func TestProjectJSONCarriesVisibilityAndMembers(t *testing.T) {
	p := Project{ID: 5, Name: "Installation", CreatedBy: 1, IsPrivate: true, SharedWithMembers: true}
	p.SetMembers([]int{3, 2})
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["visibility"] != "members" || out["isPrivate"] != true || out["name"] != "Installation" {
		t.Fatalf("unexpected json: %s", raw)
	}
	if !reflect.DeepEqual(out["memberIds"], []interface{}{float64(2), float64(3)}) {
		t.Fatalf("memberIds = %v", out["memberIds"])
	}
	if _, leaked := out["SharedWithMembers"]; leaked {
		t.Fatal("internal column must not be serialized")
	}

	circle := Project{ID: 6, Name: "Haushalt"}
	raw, _ = json.Marshal(&circle)
	_ = json.Unmarshal(raw, &out)
	if out["visibility"] != "circle" {
		t.Fatalf("pointer marshal: %s", raw)
	}
}

func TestResolveAccess(t *testing.T) {
	yes, no := true, false
	members := []int{7}
	vis := func(v ProjectVisibility) *ProjectVisibility { return &v }
	ids := func(v ...int) *[]int { return &v }

	cases := []struct {
		name        string
		req         ProjectReq
		current     ProjectVisibility
		wantVis     ProjectVisibility
		wantMembers []int
	}{
		{"nothing sent keeps members", ProjectReq{}, ProjectVisibilityMembers, ProjectVisibilityMembers, []int{7}},
		{"isPrivate=true keeps members", ProjectReq{IsPrivate: &yes}, ProjectVisibilityMembers, ProjectVisibilityMembers, []int{7}},
		{"isPrivate=false opens", ProjectReq{IsPrivate: &no}, ProjectVisibilityMembers, ProjectVisibilityCircle, nil},
		{"isPrivate=true on circle", ProjectReq{IsPrivate: &yes}, ProjectVisibilityCircle, ProjectVisibilityPrivate, nil},
		{"visibility wins over isPrivate", ProjectReq{IsPrivate: &no, Visibility: vis(ProjectVisibilityPrivate)}, ProjectVisibilityCircle, ProjectVisibilityPrivate, nil},
		{"members without list keeps list", ProjectReq{Visibility: vis(ProjectVisibilityMembers)}, ProjectVisibilityMembers, ProjectVisibilityMembers, []int{7}},
		{"members from private starts empty", ProjectReq{Visibility: vis(ProjectVisibilityMembers)}, ProjectVisibilityPrivate, ProjectVisibilityMembers, []int{}},
		{"explicit list replaces", ProjectReq{Visibility: vis(ProjectVisibilityMembers), MemberIDs: ids(8, 9)}, ProjectVisibilityMembers, ProjectVisibilityMembers, []int{8, 9}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotVis, gotMembers := c.req.ResolveAccess(c.current, members)
			if gotVis != c.wantVis || !reflect.DeepEqual(gotMembers, c.wantMembers) {
				t.Fatalf("got %q %v, want %q %v", gotVis, gotMembers, c.wantVis, c.wantMembers)
			}
		})
	}
}
