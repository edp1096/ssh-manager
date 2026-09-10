//go:build !linux

package main

import gotty "github.com/mattn/go-tty"

// Keep the existing platform implementation, notably Windows console events.
func terminalSize(tty *gotty.TTY) (int, int, error) { return tty.Size() }
func terminalResizeEvents(tty *gotty.TTY) (<-chan gotty.WINSIZE, func()) {
	return tty.SIGWINCH(), func() {}
}
