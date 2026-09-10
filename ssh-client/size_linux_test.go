//go:build linux

package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	gotty "github.com/mattn/go-tty"
	"golang.org/x/sys/unix"
)

func TestResizeDoesNotQueryOrConsumeTerminalInput(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	fd := int(master.Fd())
	if err = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	tty, err := gotty.OpenDevice(fmt.Sprintf("/dev/pts/%d", n))
	if err != nil {
		t.Fatal(err)
	}
	defer tty.Close()
	clean, err := tty.Raw()
	if err != nil {
		t.Fatal(err)
	}
	defer clean()
	inputFD := int(tty.Input().Fd())
	before, err := unix.IoctlGetTermios(inputFD, unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	sizes, stop := terminalResizeEvents(tty)
	defer stop()
	type readResult struct {
		r   rune
		err error
	}
	reads := make(chan readResult, 1)
	go func() {
		for i := 0; i < 30; i++ {
			r, err := tty.ReadRune()
			reads <- readResult{r, err}
			if err != nil {
				return
			}
		}
	}()
	for i := 0; i < 30; i++ {
		// Pixel dimensions deliberately remain zero, the old fallback trigger.
		ws := &unix.Winsize{Col: uint16(80 + i*3), Row: uint16(24 + i)}
		if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, ws); err != nil {
			t.Fatal(err)
		}
		w, h, err := terminalSize(tty)
		if err != nil || w != int(ws.Col) || h != int(ws.Row) {
			t.Fatal(w, h, err)
		}
		if err := unix.Kill(os.Getpid(), unix.SIGWINCH); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-sizes:
			if got.W != w || got.H != h {
				t.Fatal(got, w, h)
			}
		case <-time.After(time.Second):
			t.Fatal("resize event lost")
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if n, err := unix.Poll(poll, 10); err != nil || n != 0 {
			t.Fatal("unexpected terminal output/query", n, err)
		}
		select {
		case got := <-reads:
			t.Fatal("resize interrupted or consumed input", got)
		default:
		}
		if _, err := master.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-reads:
			if got.err != nil || got.r != 'x' {
				t.Fatal(got)
			}
		case <-time.After(time.Second):
			t.Fatal("keyboard input lost")
		}
	}
	after, err := unix.IoctlGetTermios(inputFD, unix.TCGETS)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("resize changed terminal modes", err)
	}
	stop()
	stop()
	if _, ok := <-sizes; ok {
		t.Fatal("resize subscription not closed")
	}
}
