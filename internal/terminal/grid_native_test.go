//go:build linux || freebsd

package terminal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestKonsoleGridVersion(t *testing.T) {
	for version, want := range map[string]bool{"konsole 23.08.5": false, "konsole 24.02.0": true, "konsole 24.01.90": false, "konsole 25.12.0": true, "broken": false} {
		if konsoleGridVersion(version) != want {
			t.Fatal(version)
		}
	}
}

func TestKonsoleGridOrderCorrection(t *testing.T) {
	step := 0
	call := func(service, path, method string, args ...string) (string, error) {
		if service != "org.kde.konsole-42" {
			t.Fatal(service)
		}
		step++
		switch step {
		case 1:
			return "(['(0)[10|12|11]'],)", nil
		case 2:
			if !strings.HasSuffix(method, "moveView") || !reflect.DeepEqual(args, []string{"12", "0", "2"}) {
				t.Fatal(method, args)
			}
			return "(true,)", nil
		case 3:
			return "(['(0)[10|11|12]'],)", nil
		}
		return "", fmt.Errorf("unexpected call")
	}
	id, err := orderKonsolePane("org.kde.konsole-42", []int{10, 11}, 11, call)
	if err != nil || id != 12 || step != 3 {
		t.Fatal(id, err, step)
	}
	for _, s := range []string{"", "(['(0)[1|]'],)", "(['(0)[1]','(2)[3]'],)", "(['(0)[1]junk'],)"} {
		if _, err := parseKonsoleHierarchy(s); err == nil {
			t.Fatal("bad hierarchy accepted", s)
		}
	}
	tree, err := parseKonsoleHierarchy("(['(0){(1)[10|11|12]|(2)[13|14|15]}'],)")
	if err != nil || len(tree.leaves()) != 6 {
		t.Fatal(tree, err)
	}
}

func TestTilixGridBindings(t *testing.T) {
	args := make([]SshClientArgument, 9)
	for i := range args {
		args[i].HostIndex = i + 1
	}
	plan, _ := PlanGrid(args, 3)
	layout, ids := tilixGridLayout(plan, "/test app/it's-manager", "/tmp/test-grid", "profile")
	byID := map[string]int{}
	for i, id := range ids {
		byID[id.TilixID] = plan[i].HostIndex
	}
	var order []int
	var visit func(map[string]any)
	visit = func(n map[string]any) {
		if n["type"] == "Terminal" {
			order = append(order, byID[n["uuid"].(string)])
			if !strings.Contains(n["overrideCommand"].(string), "'\\''") {
				t.Fatal("unquoted helper")
			}
			return
		}
		visit(n["child1"].(map[string]any))
		visit(n["child2"].(map[string]any))
	}
	visit(layout["child"].(map[string]any))
	if !reflect.DeepEqual(order, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatal(order)
	}
}

func TestKonsoleGridShapeVerification(t *testing.T) {
	leftCall := func(service, path, method string, args ...string) (string, error) {
		return "(['(0)[10|(1){11|12}]'],)", nil
	}
	if err := verifyKonsoleGrid("org.kde.konsole-42", []int{10, 11, 12}, 2, leftCall, true, true); err != nil {
		t.Fatal(err)
	}
	for _, hierarchy := range []string{"(0)[(1){10|12}|11]", "(0)[(1){10|13|16}|(2){11|14|17}|(3){12|15}]"} {
		views := []int{10, 11, 12}
		columns := 2
		if strings.Contains(hierarchy, "17") {
			views = []int{10, 11, 12, 13, 16, 14, 17, 15}
			columns = 3
		}
		call := func(service, path, method string, args ...string) (string, error) {
			return "(['" + hierarchy + "'],)", nil
		}
		if err := verifyKonsoleGrid("org.kde.konsole-42", views, columns, call, true); err != nil {
			t.Fatal(err)
		}
	}
	views := []int{10, 13, 16, 11, 12, 14, 15, 17, 18}
	for _, tc := range []struct {
		hierarchy string
		ok        bool
	}{
		{"(0){(1)[10|11|12]|(2)[13|14|15]|(3)[16|17|18]}", true},
		{"(0){(1)[10|12|11]|(2)[13|14|15]|(3)[16|17|18]}", false},
		{"(0)[(1){10|11|12}|(2){13|14|15}|(3){16|17|18}]", false},
	} {
		call := func(service, path, method string, args ...string) (string, error) {
			return "(['" + tc.hierarchy + "'],)", nil
		}
		err := verifyKonsoleGrid("org.kde.konsole-42", views, 3, call)
		if (err == nil) != tc.ok {
			t.Fatal(tc, err)
		}
	}
	call := func(service, path, method string, args ...string) (string, error) {
		return "<node><interface><method name=\"viewHierarchy\"/></interface></node>", nil
	}
	if err := konsoleGridCapabilities("org.kde.konsole-42", call); err == nil {
		t.Fatal("missing moveView accepted")
	}
}
