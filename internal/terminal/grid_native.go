//go:build linux || freebsd

package terminal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type konsoleNode struct {
	id       int
	axis     byte
	children []*konsoleNode
}

// gdbus prints one quoted hierarchy string per tab. Grid windows own one tab.
func parseKonsoleHierarchy(output string) (*konsoleNode, error) {
	quotes := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(output, -1)
	if len(quotes) != 1 {
		return nil, fmt.Errorf("unexpected Konsole tab hierarchy")
	}
	s := quotes[0][1]
	pos := 0
	integer := func() (int, error) {
		start := pos
		for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
			pos++
		}
		if start == pos {
			return 0, fmt.Errorf("invalid hierarchy ID")
		}
		return strconv.Atoi(s[start:pos])
	}
	var parse func() (*konsoleNode, error)
	parse = func() (*konsoleNode, error) {
		if pos >= len(s) {
			return nil, fmt.Errorf("incomplete hierarchy")
		}
		n := &konsoleNode{}
		branch := s[pos] == '('
		if branch {
			pos++
		}
		var err error
		n.id, err = integer()
		if err != nil {
			return nil, err
		}
		if !branch {
			return n, nil
		}
		if pos+1 >= len(s) || s[pos] != ')' || (s[pos+1] != '[' && s[pos+1] != '{') {
			return nil, fmt.Errorf("invalid splitter")
		}
		pos++
		n.axis = s[pos]
		pos++
		end := byte(']')
		if n.axis == '{' {
			end = '}'
		}
		for {
			child, e := parse()
			if e != nil {
				return nil, e
			}
			n.children = append(n.children, child)
			if pos >= len(s) {
				return nil, fmt.Errorf("unclosed splitter")
			}
			c := s[pos]
			pos++
			if c == end {
				break
			}
			if c != '|' {
				return nil, fmt.Errorf("invalid separator")
			}
		}
		return n, nil
	}
	n, err := parse()
	if err == nil && pos != len(s) {
		err = fmt.Errorf("trailing hierarchy data")
	}
	return n, err
}

func (n *konsoleNode) leaves() []int {
	if n.axis == 0 {
		return []int{n.id}
	}
	var ids []int
	for _, c := range n.children {
		ids = append(ids, c.leaves()...)
	}
	return ids
}
func (n *konsoleNode) parentOf(id int) *konsoleNode {
	for _, c := range n.children {
		if c.axis == 0 && c.id == id {
			return n
		}
		if p := c.parentOf(id); p != nil {
			return p
		}
	}
	return nil
}
func readKonsoleTree(service string, call dbusCaller) (*konsoleNode, error) {
	out, err := call(service, "/Windows/1", "org.kde.konsole.Window.viewHierarchy")
	if err != nil {
		return nil, err
	}
	return parseKonsoleHierarchy(out)
}

// Some Konsole releases insert a third same-axis split BEFORE the active view.
// Identify the new view, explicitly place it after its target, then read back.
func orderKonsolePane(service string, known []int, target int, call dbusCaller) (int, error) {
	tree, err := readKonsoleTree(service, call)
	if err != nil {
		return 0, err
	}
	seen := map[int]bool{}
	for _, id := range known {
		seen[id] = true
	}
	var added []int
	for _, id := range tree.leaves() {
		if !seen[id] {
			added = append(added, id)
		}
	}
	if len(added) != 1 || len(tree.leaves()) != len(known)+1 {
		return 0, fmt.Errorf("Konsole grid pane count changed unexpectedly")
	}
	id := added[0]
	if len(known) == 0 {
		return id, nil
	}
	parent := tree.parentOf(target)
	if parent == nil || parent != tree.parentOf(id) {
		return 0, fmt.Errorf("Konsole split target hierarchy mismatch")
	}
	desired := -1
	index := 0
	for _, c := range parent.children {
		if c.axis == 0 && c.id == id {
			continue
		}
		index++
		if c.axis == 0 && c.id == target {
			desired = index
		}
	}
	if desired < 0 {
		return 0, fmt.Errorf("Konsole grid target missing")
	}
	actual := -1
	for i, c := range parent.children {
		if c.axis == 0 && c.id == id {
			actual = i
		}
	}
	if actual != desired {
		out, e := call(service, "/Windows/1", "org.kde.konsole.Window.moveView", strconv.Itoa(id), strconv.Itoa(parent.id), strconv.Itoa(desired))
		if e != nil || out != "(true,)" {
			return 0, fmt.Errorf("Konsole grid ordering failed: %v", e)
		}
		tree, err = readKonsoleTree(service, call)
		if err != nil {
			return 0, err
		}
		p := tree.parentOf(id)
		if p == nil || desired >= len(p.children) || p.children[desired].id != id || p.children[desired-1].id != target {
			return 0, fmt.Errorf("Konsole grid ordering verification failed")
		}
	}
	return id, nil
}

var konsoleVersionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)(?:\.\d+)?\b`)

func konsoleGridVersion(output string) bool {
	m := konsoleVersionPattern.FindStringSubmatch(output)
	if m == nil {
		return false
	}
	y, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	return y > 24 || y == 24 && month >= 2
}

func equalKonsoleGrid(service string, call dbusCaller) error {
	// All calls target the dedicated process, never a user's active window.
	if !servicePattern.MatchString(service) {
		return fmt.Errorf("invalid grid service")
	}
	hierarchy, err := call(service, "/Windows/1", "org.kde.konsole.Window.viewHierarchy")
	if err != nil || !strings.ContainsAny(hierarchy, "[{") {
		return fmt.Errorf("Konsole grid hierarchy is unavailable: %v", err)
	}
	actions, err := call(service, "/konsole/MainWindow_1", "org.kde.KMainWindow.actions")
	if err != nil || !strings.Contains(actions, "equal-size-view") {
		return fmt.Errorf("Konsole equal-size-view is unavailable: %v", err)
	}
	result, err := call(service, "/konsole/MainWindow_1", "org.kde.KMainWindow.activateAction", "equal-size-view")
	if err != nil || result != "(true,)" {
		return fmt.Errorf("Konsole refused grid sizing: %v", err)
	}
	return nil
}

func verifyKonsoleGrid(service string, views []int, columns int, call dbusCaller) error {
	tree, err := readKonsoleTree(service, call)
	if err != nil {
		return err
	}
	unwrap := func(n *konsoleNode) *konsoleNode {
		for len(n.children) == 1 {
			n = n.children[0]
		}
		return n
	}
	tree = unwrap(tree)
	rows := (len(views) + columns - 1) / columns
	rowNodes := []*konsoleNode{tree}
	if rows > 1 {
		if tree.axis != '{' || len(tree.children) != rows {
			return fmt.Errorf("Konsole grid row structure mismatch")
		}
		rowNodes = tree.children
	}
	next := rows
	for row, node := range rowNodes {
		node = unwrap(node)
		count := min(columns, len(views)-row*columns)
		cells := []*konsoleNode{node}
		if count > 1 {
			if node.axis != '[' || len(node.children) != count {
				return fmt.Errorf("Konsole grid column structure mismatch")
			}
			cells = node.children
		}
		for col, cell := range cells {
			cell = unwrap(cell)
			want := views[row]
			if col > 0 {
				want = views[next]
				next++
			}
			if cell.axis != 0 || cell.id != want {
				return fmt.Errorf("Konsole grid host order mismatch")
			}
		}
	}
	return nil
}

func konsoleGridCapabilities(service string, call dbusCaller) error {
	xml, err := call(service, "/Windows/1", "org.freedesktop.DBus.Introspectable.Introspect")
	if err != nil {
		return err
	}
	for _, method := range []string{"viewHierarchy", "moveView", "setCurrentSession"} {
		if !strings.Contains(xml, `name="`+method+`"`) {
			return fmt.Errorf("Konsole grid API %s is unavailable", method)
		}
	}
	actions, err := call(service, "/konsole/MainWindow_1", "org.kde.KMainWindow.actions")
	if err != nil {
		return err
	}
	if !strings.Contains(actions, "equal-size-view") {
		return fmt.Errorf("Konsole grid sizing action is unavailable")
	}
	return nil
}
