package repo

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	stModel "donetick.com/core/internal/subtask/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestSubTasksRepo(t *testing.T) (*SubTasksRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "subtasks.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stModel.SubTask{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	return NewSubTasksRepository(db), db
}

func TestSubTaskOrderIDAcceptsLargeValues(t *testing.T) {
	var st stModel.SubTask
	if err := json.Unmarshal([]byte(`{"id":-1,"name":"x","orderId":300,"parentId":null}`), &st); err != nil {
		t.Fatalf("unmarshal orderId 300: %v", err)
	}
	if st.OrderID != 300 {
		t.Fatalf("OrderID = %d, want 300", st.OrderID)
	}
}

func TestUpdateSubtaskPersistsLargeOrderIDs(t *testing.T) {
	repo, db := newTestSubTasksRepo(t)
	const n = 200

	parent := stModel.SubTask{ID: -1, Name: "parent", OrderID: 0}
	toAdd := []stModel.SubTask{parent}
	for i := 0; i < n; i++ {
		parentID := -1
		toAdd = append(toAdd, stModel.SubTask{ID: -(i + 2), Name: "child", OrderID: i, ParentId: &parentID})
	}
	if err := repo.UpdateSubtask(context.Background(), 1, nil, toAdd); err != nil {
		t.Fatalf("UpdateSubtask: %v", err)
	}

	var children []stModel.SubTask
	if err := db.Where("chore_id = ? AND parent_id IS NOT NULL", 1).Order("order_id").Find(&children).Error; err != nil {
		t.Fatalf("query children: %v", err)
	}
	if len(children) != n {
		t.Fatalf("got %d children, want %d", len(children), n)
	}
	if last := children[n-1].OrderID; last != n-1 {
		t.Fatalf("last OrderID = %d, want %d", last, n-1)
	}
}
