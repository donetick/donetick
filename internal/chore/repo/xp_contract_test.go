package chore

import (
	chModel "donetick.com/core/internal/chore/model"
	"reflect"
	"testing"
)

func TestXPCategoryTaskContract(t *testing.T) {
	field, ok := reflect.TypeOf(chModel.Chore{}).FieldByName("XPCategoryID")
	if !ok {
		t.Fatal("task has no persisted XP category; categorized awards cannot be configured")
	}
	if field.Tag.Get("json") != "xpCategoryId" {
		t.Fatal("XP category must be exposed to clients")
	}
}
