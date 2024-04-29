package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

func handleHTML(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, html)
}

func handleQuit(w http.ResponseWriter, r *http.Request) {
	exec.Command("pkill", "ssh-client").Run()

	if server != nil {
		server.Close()
	} else {
		os.Exit(0)
	}
}

func handleOpenSession(w http.ResponseWriter, r *http.Request) {
	// params := r.URL.Query()
	// mode := params.Get("mode")

	body, err := io.ReadAll(io.Reader(r.Body))
	if err != nil {
		http.Error(w, "Request body reading failed", http.StatusInternalServerError)
		return
	}

	var arg SshArgument
	err = json.Unmarshal(body, &arg)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	openSession(arg)
}

func runServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", handleHTML)
	mux.HandleFunc("GET /quit", handleQuit)
	mux.HandleFunc("POST /open-session", handleOpenSession)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		http.ListenAndServe("localhost:11080", mux)
		server = &http.Server{Addr: "localhost:11080", Handler: http.DefaultServeMux}
		server.ListenAndServe()
		wg.Done()
	}()

	openBrowser("http://localhost:11080")

	wg.Wait()
}
