package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"ssh-client/paneexit"
)

func main() {
	dir := flag.String("f", "", "report directory")
	id := flag.Int("hi", 0, "host index")
	flag.Int("ci", 0, "category")
	flag.String("k", "", "unused key")
	readyAddress := flag.String("launch-ready-address", "", "startup address")
	readyToken := flag.String("launch-ready-token", "", "startup token")
	exitGroup := flag.String("launch-exit-group", "", "exit group")
	flag.Parse()
	guard, err := paneexit.New(*exitGroup)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := guard.BeforeProcessExit(); err != nil {
			panic(err)
		}
	}()
	fmt.Printf("SSH Manager grid test: pane %d (closes automatically)\n", *id)
	if *readyAddress != "" {
		conn, err := net.DialTimeout("tcp", *readyAddress, time.Second)
		if err != nil {
			return
		}
		json.NewEncoder(conn).Encode(*readyToken)
		conn.Close()
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(*dir, "sample")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	var info struct {
		Size, Cursor struct{ X, Y int16 }
		Attributes   uint16
		Window       struct{ Left, Top, Right, Bottom int16 }
		Maximum      struct{ X, Y int16 }
	}
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfo")
	ok, _, _ := proc.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		os.Exit(1)
	}
	data, _ := json.Marshal(struct{ Width, Height int }{int(info.Window.Right - info.Window.Left + 1), int(info.Window.Bottom - info.Window.Top + 1)})
	if err := os.WriteFile(filepath.Join(*dir, fmt.Sprintf("%d.json", *id)), data, 0600); err != nil {
		os.Exit(1)
	}
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(*dir, fmt.Sprintf("close-%d", *id))); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
