//go:build !linux

package terminal

func RunChild() bool         { return false }
func Cleanup()               {}
func Status() TerminalStatus { return TerminalStatus{Ready: true} }
