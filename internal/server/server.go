package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"ssh-manager/internal/broadcast"
	"ssh-manager/internal/browser"
	"ssh-manager/internal/filetransfer"
	"ssh-manager/internal/host"
	"ssh-manager/internal/terminal"
	"ssh-manager/pkg/model"
	"ssh-manager/pkg/utils"
)

type InitData struct {
	HostFileKEY []byte
	EmbedFiles  embed.FS
	BrowserData embed.FS
	Version     string
}

type EnterPassword struct {
	Password string `json:"password"`
}

type ChangePassword struct {
	PasswordOLD string `json:"password-old"`
	PasswordNEW string `json:"password-new"`
}

type ReorderRequest struct {
	HostList model.HostList `json:"hosts"`
}

var (
	WorkingDir     string
	Server         *http.Server
	AvailablePort  int
	WebSocketConns []int
	cmdBrowser     *exec.Cmd
	EmbedFiles     embed.FS
	BrowserData    embed.FS
	VERSION        string
	HostFileKEY    []byte
	inputBroker    = broadcast.New()
	fileManager    = filetransfer.New(func(file, id string) (model.HostInfo, error) {
		var hosts model.HostList
		if id == "" {
			return model.HostInfo{}, fmt.Errorf("Host ID is required")
		}
		if err := host.LoadHostData(file, HostFileKEY, &hosts); err != nil {
			return model.HostInfo{}, err
		}
		for _, category := range hosts.Categories {
			for _, h := range category.Hosts {
				if h.UniqueID == id {
					return h, nil
				}
			}
		}
		return model.HostInfo{}, fmt.Errorf("Host no longer exists; reload the host list")
	})
)

var applicationExitOnce sync.Once

func ExitProcess() {
	// Wait for browser refresh checking
	time.Sleep(500 * time.Millisecond)
	if len(WebSocketConns) > 0 {
		return
	}
	exitApplication()
}

func exitApplication() {
	applicationExitOnce.Do(func() {
		terminal.Cleanup()
		inputBroker.Close()
		cleanupBatchFiles()
		fileManager.Close()

		if cmdBrowser != nil && cmdBrowser.Process != nil {
			cmdBrowser.Process.Kill()
		}

		time.Sleep(100 * time.Millisecond)

		// Remove browser_data
		dataPath := filepath.FromSlash(WorkingDir + "/browser_data")
		os.RemoveAll(dataPath)

		os.Exit(0)
	})
}

func applicationExitHandler(exit func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		// Flush the acknowledgement before closing the managed application window.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() { time.Sleep(100 * time.Millisecond); exit() }()
	}
}

func FindPasswordByUUID(categories []model.HostCategory, uuid string) (password string, found bool) {
	password = ""
	found = false

	for _, c := range categories {
		for _, h := range c.Hosts {
			if h.UniqueID == uuid {
				password = h.Password
				found = true
				return password, found
			}
		}
	}

	return password, found
}

func handleConnectionWatchdog(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not open websocket connection", http.StatusBadRequest)
		return
	}
	defer ExitProcess()
	defer func() { WebSocketConns = WebSocketConns[1:] }()
	defer conn.Close()

	WebSocketConns = append(WebSocketConns, 1)
	for {
		_, _, err := conn.ReadMessage()
		// if websocket.IsCloseError(err, websocket.CloseNormalClosure) || websocket.IsCloseError(err, websocket.CloseGoingAway) {
		if err != nil {
			// fmt.Println(err)
			break
		}
	}
}

func handleEnterPassword(w http.ResponseWriter, r *http.Request) {
	var err error
	var data EnterPassword

	hosts := model.GetEmptyHostList()

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

	HostFileKEY, err = utils.GenerateKey(data.Password)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	if _, err := os.Stat(hostsFile); os.IsNotExist(err) {
		err = host.SaveHostData(hostsFile, HostFileKEY, &hosts)
		if err != nil {
			http.Error(w, "failed to create host data", http.StatusInternalServerError)
			return
		}
	}

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
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
	var hosts model.HostList

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

	hostFileKeyOLD, err := utils.GenerateKey(data.PasswordOLD)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	hostFileKeyNEW, err := utils.GenerateKey(data.PasswordNEW)
	if err != nil {
		http.Error(w, "failed to generate key", http.StatusInternalServerError)
		return
	}

	if len(hostFileKeyOLD) != len(HostFileKEY) || !bytes.Equal(hostFileKeyOLD, HostFileKEY) {
		http.Error(w, "wrong password", http.StatusBadRequest)
		return
	}

	err = host.LoadHostData(hostsFile, hostFileKeyOLD, &hosts)
	if err != nil {
		http.Error(w, "failed to to load host data", http.StatusInternalServerError)
		return
	}

	err = host.SaveHostData(hostsFile, hostFileKeyNEW, &hosts)
	if err != nil {
		http.Error(w, "failed to create host data", http.StatusInternalServerError)
		return
	}

	HostFileKEY = hostFileKeyNEW

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleGetHostPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	params := r.URL.Query()
	categoryIndex, categoryErr := strconv.Atoi(params.Get("category-idx"))
	hostIndex, hostErr := strconv.Atoi(params.Get("host-idx"))
	if categoryErr != nil || hostErr != nil || categoryIndex < 0 || hostIndex < 0 || strings.TrimSpace(params.Get("hosts-file")) == "" {
		http.Error(w, "invalid host selection", http.StatusBadRequest)
		return
	}
	var hosts model.HostList
	if err := host.LoadHostData(params.Get("hosts-file"), HostFileKEY, &hosts); err != nil {
		http.Error(w, "failed to load host data", http.StatusInternalServerError)
		return
	}
	if categoryIndex >= len(hosts.Categories) || hostIndex >= len(hosts.Categories[categoryIndex].Hosts) {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"password": hosts.Categories[categoryIndex].Hosts[hostIndex].Password})
}

func handleGetHosts(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts model.HostList

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = model.GetEmptyHostList()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(hosts)
}

func handleAddEditCategory(w http.ResponseWriter, r *http.Request) {
	var err error
	var categoryRequest model.HostCategory
	var hosts model.HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	categoryIdxSTR := strings.TrimSpace(params.Get("category-idx"))

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&categoryRequest)
	if err != nil {
		http.Error(w, "invalid host data", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = model.GetEmptyHostList()
	}

	categoryIDX := 0
	if categoryIdxSTR != "" {
		categoryIDX, err = strconv.Atoi(categoryIdxSTR)
		if err != nil {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
		if int(categoryIDX) > len(hosts.Categories)-1 {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
	}

	if categoryIdxSTR == "" {
		hosts.Categories = append(hosts.Categories, categoryRequest)
	} else {
		categoryRequest.Hosts = hosts.Categories[categoryIDX].Hosts
		hosts.Categories[categoryIDX] = categoryRequest
	}

	err = host.SaveHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		http.Error(w, "error saving host data file", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts model.HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	idxSTR := strings.TrimSpace(params.Get("idx"))

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = model.GetEmptyHostList()
	}

	if idxSTR == "" {
		http.Error(w, "require category index", http.StatusBadRequest)
		return
	} else {
		idx, _ := strconv.ParseInt(idxSTR, 10, 64)

		if int(idx) > len(hosts.Categories)-1 {
			http.Error(w, "wrong index", http.StatusBadRequest)
			return
		}

		hosts.Categories = slices.Delete(hosts.Categories, int(idx), int(idx+1))
	}

	err = host.SaveHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		http.Error(w, "error saving host data file", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleAddEditHost(w http.ResponseWriter, r *http.Request) {
	var err error
	var hostRequest model.HostRequestInfo
	var hosts model.HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	categoryIdxSTR := strings.TrimSpace(params.Get("category-idx"))
	hostIdxSTR := strings.TrimSpace(params.Get("host-idx"))

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&hostRequest)
	if err != nil {
		http.Error(w, "invalid host data", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = model.GetEmptyHostList()
	}

	categoryIDX := 0
	if categoryIdxSTR != "" {
		categoryIDX, err = strconv.Atoi(categoryIdxSTR)
		if err != nil {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
		if int(categoryIDX) > len(hosts.Categories)-1 {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
	}

	if hostIdxSTR == "" {
		hosts.Categories[categoryIDX].Hosts = append(hosts.Categories[categoryIDX].Hosts, model.HostInfo(hostRequest))
	} else {
		hostIDX, _ := strconv.ParseInt(hostIdxSTR, 10, 64)

		if strings.TrimSpace(hostRequest.PrivateKeyText) == "" && strings.TrimSpace(hostRequest.Password) == "" {
			hostRequest.PrivateKeyText = hosts.Categories[categoryIDX].Hosts[hostIDX].PrivateKeyText
			hostRequest.Password = hosts.Categories[categoryIDX].Hosts[hostIDX].Password
		}

		hosts.Categories[categoryIDX].Hosts[hostIDX] = model.HostInfo(hostRequest)
	}

	err = host.SaveHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		// fmt.Println(err)
		http.Error(w, "error saving host data file", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleReorderHosts(w http.ResponseWriter, r *http.Request) {
	var err error
	var body ReorderRequest
	var hostsOLD, hostsNEW model.HostList

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = host.LoadHostData(hostsFile, HostFileKEY, &hostsOLD)
	if err != nil {
		http.Error(w, "host-file not exists", http.StatusBadRequest)
		return
	}

	bodyJSON, err := io.ReadAll(io.Reader(r.Body))
	if err != nil {
		http.Error(w, "Request body reading failed", http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(bodyJSON, &body.HostList)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}
	hostsNEW = body.HostList

	for i, nc := range hostsNEW.Categories {
		// fmt.Println(nc.Name)
		for j, nh := range nc.Hosts {
			password, found := FindPasswordByUUID(hostsOLD.Categories, nh.UniqueID)
			if found {
				hostsNEW.Categories[i].Hosts[j].Password = password
			}
		}
	}

	err = host.SaveHostData(hostsFile, HostFileKEY, &hostsNEW)
	if err != nil {
		http.Error(w, "failed to save host data", http.StatusInternalServerError)
		return
	}

	result := map[string]string{"message": "success"}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(result)
}

func handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts model.HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	categoryIdxSTR := strings.TrimSpace(params.Get("category-idx"))
	hostIdxSTR := strings.TrimSpace(params.Get("host-idx"))

	err = host.LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = model.GetEmptyHostList()
	}

	categoryIDX := 0
	if categoryIdxSTR != "" {
		categoryIDX, err = strconv.Atoi(categoryIdxSTR)
		if err != nil {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
		if int(categoryIDX) > len(hosts.Categories)-1 {
			http.Error(w, "wrong category index", http.StatusBadRequest)
			return
		}
	}

	if hostIdxSTR == "" {
		http.Error(w, "require host index", http.StatusBadRequest)
		return
	} else {
		hostIDX, _ := strconv.ParseInt(hostIdxSTR, 10, 64)

		if int(hostIDX) > len(hosts.Categories[categoryIDX].Hosts)-1 {
			http.Error(w, "wrong index", http.StatusBadRequest)
			return
		}

		hosts.Categories[categoryIDX].Hosts = slices.Delete(hosts.Categories[categoryIDX].Hosts, int(hostIDX), int(hostIDX+1))
	}

	err = host.SaveHostData(hostsFile, HostFileKEY, &hosts)
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

	var arg terminal.SshClientArgument
	err = json.Unmarshal(body, &arg)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	arg.NewWindow = false
	if windowMode == "new_window" {
		arg.NewWindow = true
	}

	arg.SplitVertical = false
	if windowMode == "split_vertical" {
		arg.SplitVertical = true
	}

	arg.HostFileKEY = HostFileKEY
	if status := terminal.Status(); !status.Ready {
		http.Error(w, status.Message, http.StatusServiceUnavailable)
		return
	}
	var hosts model.HostList
	if err := host.LoadHostData(arg.HostsFile, HostFileKEY, &hosts); err != nil {
		http.Error(w, "Cannot load hosts", http.StatusBadRequest)
		return
	}
	if arg.CategoryIndex < 1 || arg.CategoryIndex > len(hosts.Categories) || arg.HostIndex < 1 || arg.HostIndex > len(hosts.Categories[arg.CategoryIndex-1].Hosts) {
		http.Error(w, "Invalid host index", http.StatusBadRequest)
		return
	}
	h := hosts.Categories[arg.CategoryIndex-1].Hosts[arg.HostIndex-1]
	arg.RelayAddress, arg.RelayToken, err = inputBroker.Issue(fmt.Sprintf("%s — %s@%s:%d", h.Name, h.Username, h.Address, h.Port))
	if err != nil {
		http.Error(w, "Cannot start input relay", http.StatusServiceUnavailable)
		return
	}

	if err := terminal.OpenSession(arg); err != nil {
		inputBroker.Revoke(arg.RelayToken)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleTerminalStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(terminal.Status())
}

func handleWindowSize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(browser.LoadWindowSize(WorkingDir))
		return
	}
	if r.Method == http.MethodPatch {
		var settings struct {
			Side  string `json:"panel-side"`
			Theme string `json:"theme"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&settings) != nil {
			http.Error(w, "Invalid panel side", 400)
			return
		}
		var err error
		if settings.Theme != "" && settings.Side == "" {
			err = browser.SaveTheme(WorkingDir, settings.Theme)
		} else if settings.Theme == "" {
			err = browser.SavePanelSide(WorkingDir, settings.Side)
		} else {
			err = fmt.Errorf("update one window preference at a time")
		}
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var size browser.WindowSize
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(r.Body).Decode(&size); err != nil || !size.Valid() {
		http.Error(w, "invalid window size", http.StatusBadRequest)
		return
	}
	if err := browser.SaveWindowSize(WorkingDir, size); err != nil {
		http.Error(w, "failed to save window size", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleOpenRepository(w http.ResponseWriter, r *http.Request) {
	if err := browser.OpenRepository(); err != nil {
		http.Error(w, "Could not open the default browser: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(VERSION))
}

func handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	fname := r.URL.Path[1:] // remove first slash
	if strings.HasPrefix(fname, "templates/") {
		http.NotFound(w, r)
		return
	}

	if fname == "" {
		fname = "index.html"
	}

	file, err := EmbedFiles.ReadFile("web/" + fname)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fext := filepath.Ext(fname)[1:]
	if fname == "index.html" {
		theme := browser.LoadWindowSize(WorkingDir).Theme
		if theme != "light" {
			theme = "dark"
		}
		file = []byte(strings.Replace(string(file), `<html lang="en" data-theme="dark">`, `<html lang="en" data-theme="`+theme+`">`, 1))
		w.Header().Set("Cache-Control", "no-store")
	}
	switch fext {
	case "js":
		w.Header().Set("Content-Type", "text/javascript")
	default:
		w.Header().Set("Content-Type", "text/"+fext)
	}

	w.Write(file)
}

func RunServer(misc InitData) {
	var err error

	WorkingDir, _, err = utils.GetCWD()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}
	if VERSION == "dev" {
		fmt.Println("WorkingDir:", WorkingDir)
	}

	EmbedFiles = misc.EmbedFiles
	BrowserData = misc.BrowserData
	VERSION = misc.Version
	HostFileKEY = misc.HostFileKEY

	AvailablePort, err = utils.GetAvailablePort()
	if err != nil {
		panic(err)
	}
	if VERSION == "dev" {
		fmt.Printf("available port: %d\n", AvailablePort)
	}

	listen := "localhost:" + strconv.Itoa(AvailablePort)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /application/exit", applicationExitHandler(exitApplication))
	fileManager.Register(mux)

	mux.HandleFunc("GET /connection-watchdog", handleConnectionWatchdog)
	mux.HandleFunc("POST /enter-password", handleEnterPassword)
	mux.HandleFunc("PUT /host-file-password", handleChangeHostFilePassword)
	mux.HandleFunc("GET /hosts", handleGetHosts)
	mux.HandleFunc("GET /host-password", handleGetHostPassword)
	mux.HandleFunc("POST /categories", handleAddEditCategory)
	mux.HandleFunc("DELETE /categories", handleDeleteCategory)
	mux.HandleFunc("POST /hosts", handleAddEditHost)
	mux.HandleFunc("PATCH /hosts", handleReorderHosts)
	mux.HandleFunc("DELETE /hosts", handleDeleteHost)
	mux.HandleFunc("DELETE /hosts/selected", handleDeleteSelectedHosts)
	mux.HandleFunc("POST /session/open", handleOpenSession)
	mux.HandleFunc("POST /session/batch", handleOpenBatch)
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		mux.HandleFunc(method+" /connection-groups", handleConnectionGroups)
	}
	mux.HandleFunc("GET /session/broadcast", inputBroker.Handler)
	mux.HandleFunc("POST /session/broadcast", inputBroker.Handler)
	mux.HandleFunc("GET /terminal/status", handleTerminalStatus)
	mux.HandleFunc("GET /version", handleGetVersion)
	mux.HandleFunc("POST /repository/open", handleOpenRepository)
	mux.HandleFunc("POST /window-size", handleWindowSize)
	mux.HandleFunc("GET /window-size", handleWindowSize)
	mux.HandleFunc("PATCH /window-size", handleWindowSize)
	mux.HandleFunc("GET /", handleStaticFiles)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		Server = &http.Server{Addr: listen, Handler: mux}
		err := Server.ListenAndServe()
		if err != nil {
			if err.Error() != "http: Server closed" {
				fmt.Println("err server running:", err)
			}
			ExitProcess()
		}
		wg.Done()
	}()

	cmdBrowser, _ = browser.OpenBrowser("http://"+listen, BrowserData)

	wg.Wait()
}
