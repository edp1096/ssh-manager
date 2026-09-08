package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type WindowSize struct {
	Width    int             `json:"width"`
	Height   int             `json:"height"`
	Position *WindowPosition `json:"position,omitempty"`
}

type WindowPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

var windowSizeMu sync.Mutex

func (s WindowSize) Valid() bool {
	if s.Width < 320 || s.Width > 16384 || s.Height < 240 || s.Height > 16384 {
		return false
	}
	return s.Position == nil || (s.Position.X >= -65536 && s.Position.X <= 65536 && s.Position.Y >= -65536 && s.Position.Y <= 65536)
}

func LoadWindowSize(dir string) WindowSize {
	windowSizeMu.Lock()
	defer windowSizeMu.Unlock()
	var size WindowSize
	data, err := os.ReadFile(filepath.Join(dir, "window.json"))
	if err != nil || json.Unmarshal(data, &size) != nil || !size.Valid() {
		return WindowSize{Width: 720, Height: 520}
	}
	return size
}

func SaveWindowSize(dir string, size WindowSize) error {
	if !size.Valid() {
		return fmt.Errorf("invalid window size")
	}
	windowSizeMu.Lock()
	defer windowSizeMu.Unlock()
	data, err := json.Marshal(size)
	if err != nil {
		return err
	}
	// Replace only after the complete new settings have been written.
	file, err := os.CreateTemp(dir, ".window-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, "window.json"))
}
