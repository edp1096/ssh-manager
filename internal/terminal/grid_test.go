package terminal

import (
	"math"
	"strings"
	"testing"
)

func TestGridPlanGeometry(t *testing.T) {
	for _, n := range []int{1, 2, 4, 5, 6, 9, 32} {
		for columns := 1; columns <= 32; columns++ {
			args := make([]SshClientArgument, n)
			for i := range args {
				args[i].HostIndex = i + 1
			}
			plan, err := PlanGrid(args, columns)
			if err != nil {
				t.Fatal(err)
			}
			type rect struct{ x, y, w, h float64 }
			cells := []rect{{0, 0, 1, 1}}
			for i := 1; i < n; i++ {
				a := plan[i]
				if a.GridTarget < 1 || a.GridTarget > i {
					t.Fatal(a)
				}
				r := cells[a.GridTarget-1]
				next := r
				if a.SplitVertical {
					r.h *= 1 - a.SplitSize
					next.y += r.h
					next.h -= r.h
				} else {
					r.w *= 1 - a.SplitSize
					next.x += r.w
					next.w -= r.w
				}
				cells[a.GridTarget-1] = r
				cells = append(cells, next)
			}
			rows := (n + columns - 1) / columns
			for i, a := range plan {
				original := a.HostIndex - 1
				row, col := original/columns, original%columns
				count := min(columns, n-row*columns)
				r := cells[i]
				if math.Abs(r.x-float64(col)/float64(count)) > 1e-7 || math.Abs(r.y-float64(row)/float64(rows)) > 1e-7 || math.Abs(r.w-1/float64(count)) > 1e-7 || math.Abs(r.h-1/float64(rows)) > 1e-7 {
					t.Fatalf("%d hosts/%d cols host %d: %+v", n, columns, a.HostIndex, r)
				}
			}
			wt, err := windowsBatchArguments("ssh-client.exe", plan)
			if err != nil {
				t.Fatal(err)
			}
			if n > 1 && !strings.Contains(strings.Join(wt, " "), "--size") {
				t.Fatal("missing grid sizes")
			}
		}
	}
	if _, err := PlanGrid(nil, 3); err == nil {
		t.Fatal("empty grid accepted")
	}
}
