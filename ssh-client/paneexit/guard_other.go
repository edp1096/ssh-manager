//go:build !windows

package paneexit

type Guard struct{}

func New(group string) (*Guard, error)    { return &Guard{}, nil }
func (g *Guard) BeforeProcessExit() error { return nil }
