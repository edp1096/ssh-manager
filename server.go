package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type EnterPassword struct {
	Password string `json:"password"`
}

type ChangePassword struct {
	PasswordOLD string `json:"password-old"`
	PasswordNEW string `json:"password-new"`
}

func handleConnectionWatchdog(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer exitProcess()
	defer conn.Close()

	for {
		_, m, err := conn.ReadMessage()
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			break
		}

		if strings.Contains(string(m), "document title|") {
			ms := strings.Split(string(m), "|")
			browserWindowTitle = ms[1]
			continue
		}
	}
}

func handleEnterPassword(w http.ResponseWriter, r *http.Request) {
	var err error
	var data EnterPassword
	var hosts []HostInfo

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&data)
	if err != nil {
		http.Error(w, "invalid request data", http.StatusBadRequest)
		return
	}

	if data.Password == "" {
		http.Error(w, "empty password", http.StatusBadRequest)
		return
	}

	hostFileKEY, err = generateKey(data.Password)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	if _, err := os.Stat(hostsFile); os.IsNotExist(err) {
		err = saveHostData(hostsFile, hostFileKEY, &hosts)
		if err != nil {
			http.Error(w, "failed to create host data", http.StatusInternalServerError)
			return
		}
	}

	err = loadHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		http.Error(w, "failed to to load host data", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleChangeHostFilePassword(w http.ResponseWriter, r *http.Request) {
	var err error
	var data ChangePassword
	var hosts []HostInfo

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&data)
	if err != nil {
		http.Error(w, "invalid request data", http.StatusBadRequest)
		return
	}

	if data.PasswordOLD == "" || data.PasswordNEW == "" {
		http.Error(w, "empty password", http.StatusBadRequest)
		return
	}

	hostFileKeyOLD, err := generateKey(data.PasswordOLD)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	hostFileKeyNEW, err := generateKey(data.PasswordNEW)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	if len(hostFileKeyOLD) != len(hostFileKEY) || !bytes.Equal(hostFileKeyOLD, hostFileKEY) {
		http.Error(w, "wrong password", http.StatusBadRequest)
		return
	}

	err = loadHostData(hostsFile, hostFileKeyOLD, &hosts)
	if err != nil {
		http.Error(w, "failed to to load host data", http.StatusInternalServerError)
		return
	}

	err = saveHostData(hostsFile, hostFileKeyNEW, &hosts)
	if err != nil {
		http.Error(w, "failed to create host data", http.StatusInternalServerError)
		return
	}

	hostFileKEY = hostFileKeyNEW

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleGetHosts(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts []HostInfo

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = loadHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		hosts = []HostInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(hosts)
}

func handleAddEditHost(w http.ResponseWriter, r *http.Request) {
	var err error
	var hostsRequest HostRequestInfo
	var hosts []HostInfo

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	idxSTR := strings.TrimSpace(params.Get("idx"))

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&hostsRequest)
	if err != nil {
		http.Error(w, "invalid host data", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = loadHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		hosts = []HostInfo{}
	}

	if idxSTR == "" {
		hosts = append(hosts, HostInfo(hostsRequest))
	} else {
		idx, _ := strconv.ParseInt(idxSTR, 10, 64)

		if strings.TrimSpace(hostsRequest.PrivateKeyText) == "" && strings.TrimSpace(hostsRequest.Password) == "" {
			hostsRequest.PrivateKeyText = hosts[idx].PrivateKeyText
			hostsRequest.Password = hosts[idx].Password
		}

		hosts[idx] = HostInfo(hostsRequest)
	}

	err = saveHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		http.Error(w, "error saving host data file", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts []HostInfo

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	idxSTR := strings.TrimSpace(params.Get("idx"))

	err = loadHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		hosts = []HostInfo{}
	}

	if idxSTR == "" {
		http.Error(w, "require idx", http.StatusBadRequest)
		return
	} else {
		idx, _ := strconv.ParseInt(idxSTR, 10, 64)

		if int(idx) > len(hosts)-1 {
			http.Error(w, "wrong index", http.StatusBadRequest)
			return
		}

		hosts = slices.Delete(hosts, int(idx), int(idx+1))
	}

	err = saveHostData(hostsFile, hostFileKEY, &hosts)
	if err != nil {
		http.Error(w, "error saving host data file", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleOpenSession(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	windowMode := params.Get("window-mode")

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

	newWindow := false
	if windowMode == "new_window" {
		newWindow = true
	}

	openSession(arg, newWindow)
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

	fext := filepath.Ext(fname)[1:]
	switch fext {
	case "js":
		w.Header().Set("Content-Type", "text/javascript")
	default:
		w.Header().Set("Content-Type", "text/"+fext)
	}

	w.Write(file)
}

func runServer() {
	listen := "localhost:11080"

	mux := http.NewServeMux()

	mux.HandleFunc("GET /connection-watchdog", handleConnectionWatchdog)
	mux.HandleFunc("POST /enter-password", handleEnterPassword)
	mux.HandleFunc("PUT /host-file-password", handleChangeHostFilePassword)
	mux.HandleFunc("GET /hosts", handleGetHosts)
	mux.HandleFunc("POST /hosts", handleAddEditHost)
	mux.HandleFunc("DELETE /hosts", handleDeleteHost)
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
