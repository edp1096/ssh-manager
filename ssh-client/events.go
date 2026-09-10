package main

import (
	"fmt"
	"io"

	gotty "github.com/mattn/go-tty"
	"golang.org/x/crypto/ssh"
)

// func clearBuffer(tty *gotty.TTY) {
// 	// Clear tty buffer
// 	for {
// 		_, _ = tty.ReadRune()
// 		if !tty.Buffered() {
// 			break
// 		}
// 	}
// }

func setResizeControl(sess *ssh.Session, tty *gotty.TTY) func() {
	sizes, stop := terminalResizeEvents(tty)
	go func() {
		for ws := range sizes {
			sess.WindowChange(ws.H, ws.W)
		}
	}()
	return stop
}

func setEventControl(pw io.Writer, tty *gotty.TTY, onEnd func()) {
	go func() {
		defer onEnd()
		var b []byte
		for {
			r, err := tty.ReadRune()
			if err != nil {
				fmt.Println("tty.ReadRune:", err)
				return
			}

			if r == rune(0) {
				continue
			}

			b = append(b, []byte(string(r))...)

			if !tty.Buffered() {
				switch string(b) {
				case string([]byte{27, 91, 72}): // Home
					b = []byte("\x1b[1~")
				case string([]byte{27, 91, 70}): // End
					b = []byte("\x1b[4~")
				}

				if _, err := pw.Write(b); err != nil {
					return
				}

				b = nil
				continue
			}
		}
	}()
}
