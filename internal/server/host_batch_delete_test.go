package server

import (
	"reflect"
	"ssh-manager/pkg/model"
	"testing"
)

func TestRemoveSelectedHosts(t *testing.T) {
	list := model.HostList{Categories: []model.HostCategory{
		{Name: "First", Hosts: []model.HostInfo{{UniqueID: "a"}, {UniqueID: "b", Password: "keep"}}},
		{Name: "Second", Hosts: []model.HostInfo{{UniqueID: "c"}}},
	}}
	for _, ids := range [][]string{nil, {"a", "missing"}, {"a", "a"}, {""}} {
		before := model.HostList{Categories: append([]model.HostCategory(nil), list.Categories...)}
		if err := removeSelectedHosts(&list, ids); err == nil {
			t.Fatal("invalid selection accepted", ids)
		}
		if !reflect.DeepEqual(before, list) {
			t.Fatal("invalid selection changed hosts")
		}
	}
	if err := removeSelectedHosts(&list, []string{"c", "a"}); err != nil {
		t.Fatal(err)
	}
	if len(list.Categories) != 2 || len(list.Categories[0].Hosts) != 1 || len(list.Categories[1].Hosts) != 0 {
		t.Fatal(list)
	}
	if list.Categories[0].Hosts[0].Password != "keep" {
		t.Fatal("unselected host changed")
	}
}
