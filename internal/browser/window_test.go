package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPanelSidePreservesWindowGeometry(t *testing.T) {
	dir := t.TempDir()
	size := WindowSize{Width: 1100, Height: 800, Position: &WindowPosition{X: -200, Y: 60}}
	if err := SaveWindowSize(dir, size); err != nil {
		t.Fatal(err)
	}
	if err := SavePanelSide(dir, "right"); err != nil {
		t.Fatal(err)
	}
	expected := size
	expected.PanelSide = "right"
	if got := LoadWindowSize(dir); !reflect.DeepEqual(got, expected) {
		t.Fatal(got)
	}
	size.Width = 1200
	if err := SaveWindowSize(dir, size); err != nil {
		t.Fatal(err)
	}
	expected.Width = 1200
	if got := LoadWindowSize(dir); !reflect.DeepEqual(got, expected) {
		t.Fatal(got)
	}
	if err := SavePanelSide(dir, "invalid"); err == nil {
		t.Fatal("invalid side accepted")
	}
	if err := SavePanelSide(dir, "left"); err != nil {
		t.Fatal(err)
	}
	if got := LoadWindowSize(dir); got.PanelSide != "left" || got.Width != 1200 {
		t.Fatal(got)
	}
}

func TestThemePersistence(t *testing.T) {
	dir := t.TempDir()
	if err := SaveTheme(dir, "light"); err != nil {
		t.Fatal(err)
	}
	if err := SavePanelSide(dir, "right"); err != nil {
		t.Fatal(err)
	}
	if err := SaveWindowSize(dir, WindowSize{Width: 1000, Height: 700}); err != nil {
		t.Fatal(err)
	}
	got := LoadWindowSize(dir)
	if got.Theme != "light" || got.PanelSide != "right" || got.Width != 1000 {
		t.Fatal(got)
	}
	if err := SaveTheme(dir, "invalid"); err == nil {
		t.Fatal("invalid theme accepted")
	}
	if err := SaveTheme(dir, "dark"); err != nil {
		t.Fatal(err)
	}
	if got := LoadWindowSize(dir); got.Theme != "dark" || got.PanelSide != "right" {
		t.Fatal(got)
	}
}

func TestWindowSizePersistence(t *testing.T) {
	dir := t.TempDir()
	defaultSize := WindowSize{Width: 720, Height: 520}
	if got := LoadWindowSize(dir); got != defaultSize {
		t.Fatalf("missing settings: %+v", got)
	}
	for _, size := range []WindowSize{{Width: 960, Height: 720}, {Width: 1200, Height: 800}} {
		if err := SaveWindowSize(dir, size); err != nil {
			t.Fatal(err)
		}
		if got := LoadWindowSize(dir); got != size {
			t.Fatalf("saved %+v, loaded %+v", size, got)
		}
	}
	for _, size := range []WindowSize{{}, {Width: -1, Height: 520}, {Width: 720, Height: 100}, {Width: 20000, Height: 800}} {
		if err := SaveWindowSize(dir, size); err == nil {
			t.Fatalf("accepted invalid size: %+v", size)
		}
	}
	if got := LoadWindowSize(dir); got != (WindowSize{Width: 1200, Height: 800}) {
		t.Fatalf("invalid update changed saved size: %+v", got)
	}
	for _, data := range []string{`broken`, `{"width":10,"height":520}`, `{}`} {
		if err := os.WriteFile(filepath.Join(dir, "window.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if got := LoadWindowSize(dir); got != defaultSize {
			t.Fatalf("invalid settings: %+v", got)
		}
	}
}

func TestWindowPositionPersistence(t *testing.T) {
	dir := t.TempDir()
	for _, position := range []WindowPosition{{X: 0, Y: 0}, {X: 400, Y: 250}, {X: -1920, Y: -300}} {
		size := WindowSize{Width: 960, Height: 720, Position: &position}
		if err := SaveWindowSize(dir, size); err != nil {
			t.Fatal(err)
		}
		if got := LoadWindowSize(dir); !reflect.DeepEqual(got, size) {
			t.Fatalf("saved %+v, loaded %+v", size, got)
		}
	}
	if err := SaveWindowSize(dir, WindowSize{Width: 960, Height: 720, Position: &WindowPosition{X: 1000000}}); err == nil {
		t.Fatal("accepted invalid position")
	}
}
