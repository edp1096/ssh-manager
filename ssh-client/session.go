package main

import (
	"fmt"
	"log"
	"os"
	"time"

	gotty "github.com/mattn/go-tty"
	"golang.org/x/crypto/ssh"
)

func openSession() (err error) {
	config := &ssh.ClientConfig{
		User:            host.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(host.Password)},
		Timeout:         5 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if host.Password == "" {
		signer, err := setSigner([]byte(host.PrivateKeyText))
		if err != nil {
			panic(err)
		}

		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	hostport := fmt.Sprintf("%s:%d", host.Address, host.Port)
	conn, err := ssh.Dial("tcp", hostport, config)
	if err != nil {
		return fmt.Errorf("ssh.Dial %v: %v", hostport, err)
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("conn.NewSession: %v", err)
	}
	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}

	tty, err := gotty.Open()
	if err != nil {
		return fmt.Errorf("tty.Open: %v", err)
	}
	defer tty.Close()

	termType := "linux"
	w, h, err := tty.Size()
	if err != nil {
		w, h = 0, 0
	}

	clean, err := tty.Raw()
	if err != nil {
		log.Fatal(err)
	}
	defer clean()

	err = sess.RequestPty(termType, h, w, modes)
	if err != nil {
		return fmt.Errorf("sess.RequestPty: %s", err)
	}

	pw, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("sess.StdinPipe: %v", err)
	}

	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	err = sess.Shell()
	if err != nil {
		return fmt.Errorf("sess.Shell: %v", err)
	}

	setResizeControl(sess, tty, pw, w, h)
	setEventControl(pw, tty)

	sess.Wait()

	return nil
}