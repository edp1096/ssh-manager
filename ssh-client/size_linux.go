//go:build linux

package main

import (
	"os"
	"os/signal"
	"sync"

	gotty "github.com/mattn/go-tty"
	"golang.org/x/sys/unix"
)

// Only character dimensions are needed for SSH. Do not call go-tty's Size or
// SIGWINCH here: its pixel fallback queries and reads the shared input stream.
func terminalSize(tty *gotty.TTY) (int, int, error) {
	return terminalCellSize(int(tty.Output().Fd()))
}

func terminalCellSize(fd int) (int, int, error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}

func terminalResizeEvents(tty *gotty.TTY) (<-chan gotty.WINSIZE, func()) {
	return watchTerminalResize(int(tty.Output().Fd()))
}

func watchTerminalResize(fd int) (<-chan gotty.WINSIZE, func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGWINCH)
	sizes := make(chan gotty.WINSIZE, 1)
	done, stopped := make(chan struct{}), make(chan struct{})
	var once sync.Once
	go func() {
		defer close(stopped)
		defer close(sizes)
		for {
			select {
			case <-done:
				return
			case <-signals:
				w, h, err := terminalCellSize(fd)
				if err != nil || w <= 0 || h <= 0 {
					continue
				}
				// Keep the newest size during rapid resize/maximize events.
				select {
				case <-sizes:
				default:
				}
				select {
				case sizes <- gotty.WINSIZE{W: w, H: h}:
				case <-done:
					return
				}
			}
		}
	}()
	return sizes, func() { once.Do(func() { signal.Stop(signals); close(done); <-stopped }) }
}
