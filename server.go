package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
)

func handleConnectionWatchdog(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer server.Close()
	defer conn.Close()

	for {
		_, _, err := conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			break
		}
	}
}

func handleGetHosts(w http.ResponseWriter, r *http.Request) {
	var err error

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	key := []byte("0123456789!#$%^&*()abcdefghijklm")
	var hosts []HostInfo

	err = loadHostData(hostsFile, key, &hosts)
	if err != nil {
		http.Error(w, "error loading host data file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(hosts)
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

func handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	fname := r.URL.Path[1:] // remove first slash

	if fname == "" {
		fname = "index.html"
	}

	file, err := embedFiles.ReadFile("html/" + fname)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fext := filepath.Ext(fname)
	switch fext {
	case "css":
		w.Header().Set("Content-Type", "text/css")
	case "js":
		w.Header().Set("Content-Type", "text/javascript")
	}

	w.Write(file)
}

func runServer() {
	listen := "localhost:11080"

	mux := http.NewServeMux()

	mux.HandleFunc("GET /connection-watchdog", handleConnectionWatchdog)
	mux.HandleFunc("GET /hosts", handleGetHosts)
	mux.HandleFunc("POST /session/open", handleOpenSession)
	mux.HandleFunc("/", handleStaticFiles)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		server = &http.Server{Addr: listen, Handler: mux}
		err := server.ListenAndServe()
		if err != nil {
			if err.Error() != "http: Server closed" {
				fmt.Println("err server running:", err)
			}
			exitProcess()
		}
		wg.Done()
	}()

	openBrowser("http://" + listen)

	wg.Wait()
}
