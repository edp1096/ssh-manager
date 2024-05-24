package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"ssh-manager/pkg/utils"
)

type EnterPassword struct {
	Password string `json:"password"`
}

type ChangePassword struct {
	PasswordOLD string `json:"password-old"`
	PasswordNEW string `json:"password-new"`
}

type ReorderRequest struct {
	HostList HostList `json:"hosts"`
}

var WebSocketConns []int

func getAvailablePort() (port int, err error) {
	portBegin := 10000
	portEnd := 50000

	source := mrand.NewSource(time.Now().UnixNano())
	randGen := mrand.New(source)

	for {
		port = randGen.Intn(portEnd-portBegin) + portBegin
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			if VERSION == "dev" {
				fmt.Printf("available port: %d\n", port)
			}
			break
		}
	}

	return
}

func ExitProcess() {
	// Wait for browser refresh checking
	time.Sleep(500 * time.Millisecond)
	if len(WebSocketConns) > 0 {
		return
	}

	CmdBrowser.Process.Kill()

	time.Sleep(100 * time.Millisecond)

	// Remove browser_data
	dataPath := filepath.FromSlash(BinaryPath + "/browser_data")
	os.RemoveAll(dataPath)

	os.Exit(0)
}

func FindPasswordByUUID(categories []HostCategory, uuid string) (password string, found bool) {
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

func SaveHostData(fileName string, key []byte, data interface{}) error {
	var buf bytes.Buffer
	iv := make([]byte, aes.BlockSize)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	_, err = io.ReadFull(crand.Reader, iv)
	if err != nil {
		return err
	}

	_, err = file.Write(iv)
	if err != nil {
		return err
	}

	stream := cipher.NewCFBEncrypter(block, iv)

	writer := &cipher.StreamWriter{S: stream, W: file}
	_, err = writer.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func LoadHostData(fileName string, key []byte, decryptedData interface{}) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("loadHostData/open: %s", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("loadHostData/Stat: %s", err)
	}

	encryptedData := make([]byte, fileInfo.Size())
	_, err = io.ReadFull(file, encryptedData)
	if err != nil {
		return fmt.Errorf("loadHostData/ReadFull: %s", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("loadHostData/NewCipher: %s", err)
	}
	iv := encryptedData[:aes.BlockSize]
	stream := cipher.NewCFBDecrypter(block, iv)

	encryptedData = encryptedData[aes.BlockSize:]

	reader := cipher.StreamReader{S: stream, R: bytes.NewReader(encryptedData)}
	decoder := gob.NewDecoder(&reader)
	err = decoder.Decode(decryptedData)
	if err != nil {
		return fmt.Errorf("loadHostData/Decode: %s", err)
	}

	return nil
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

	hosts := HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}

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
		err = SaveHostData(hostsFile, HostFileKEY, &hosts)
		if err != nil {
			http.Error(w, "failed to create host data", http.StatusInternalServerError)
			return
		}
	}

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
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
	var hosts HostList

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

	err = LoadHostData(hostsFile, hostFileKeyOLD, &hosts)
	if err != nil {
		http.Error(w, "failed to to load host data", http.StatusInternalServerError)
		return
	}

	err = SaveHostData(hostsFile, hostFileKeyNEW, &hosts)
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

func handleGetHosts(w http.ResponseWriter, r *http.Request) {
	var err error
	var hosts HostList

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(hosts)
}

func handleAddEditCategory(w http.ResponseWriter, r *http.Request) {
	var err error
	var categoryRequest HostCategory
	var hosts HostList

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

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}
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

	err = SaveHostData(hostsFile, HostFileKEY, &hosts)
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
	var hosts HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	idxSTR := strings.TrimSpace(params.Get("idx"))

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}
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

	err = SaveHostData(hostsFile, HostFileKEY, &hosts)
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
	var hostRequest HostRequestInfo
	var hosts HostList

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

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}
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
		hosts.Categories[categoryIDX].Hosts = append(hosts.Categories[categoryIDX].Hosts, HostInfo(hostRequest))
	} else {
		hostIDX, _ := strconv.ParseInt(hostIdxSTR, 10, 64)

		if strings.TrimSpace(hostRequest.PrivateKeyText) == "" && strings.TrimSpace(hostRequest.Password) == "" {
			hostRequest.PrivateKeyText = hosts.Categories[categoryIDX].Hosts[hostIDX].PrivateKeyText
			hostRequest.Password = hosts.Categories[categoryIDX].Hosts[hostIDX].Password
		}

		hosts.Categories[categoryIDX].Hosts[hostIDX] = HostInfo(hostRequest)
	}

	err = SaveHostData(hostsFile, HostFileKEY, &hosts)
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
	var hostsOLD, hostsNEW HostList

	params := r.URL.Query()
	hostsFile := params.Get("hosts-file")
	if strings.TrimSpace(hostsFile) == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	err = LoadHostData(hostsFile, HostFileKEY, &hostsOLD)
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

	err = SaveHostData(hostsFile, HostFileKEY, &hostsNEW)
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
	var hosts HostList

	params := r.URL.Query()
	hostsFile := strings.TrimSpace(params.Get("hosts-file"))
	if hostsFile == "" {
		http.Error(w, "require host-file", http.StatusBadRequest)
		return
	}

	categoryIdxSTR := strings.TrimSpace(params.Get("category-idx"))
	hostIdxSTR := strings.TrimSpace(params.Get("host-idx"))

	err = LoadHostData(hostsFile, HostFileKEY, &hosts)
	if err != nil {
		hosts = HostList{Categories: []HostCategory{{Name: "Default", Hosts: []HostInfo{}}}}
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

	err = SaveHostData(hostsFile, HostFileKEY, &hosts)
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

	file, err := EmbedFiles.ReadFile("html/" + fname)
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

func RunServer() {
	var err error

	AvailablePort, err = getAvailablePort()
	if err != nil {
		panic(err)
	}

	listen := "localhost:" + strconv.Itoa(AvailablePort)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /connection-watchdog", handleConnectionWatchdog)
	mux.HandleFunc("POST /enter-password", handleEnterPassword)
	mux.HandleFunc("PUT /host-file-password", handleChangeHostFilePassword)
	mux.HandleFunc("GET /hosts", handleGetHosts)
	mux.HandleFunc("POST /categories", handleAddEditCategory)
	mux.HandleFunc("DELETE /categories", handleDeleteCategory)
	mux.HandleFunc("POST /hosts", handleAddEditHost)
	mux.HandleFunc("PATCH /hosts", handleReorderHosts)
	mux.HandleFunc("DELETE /hosts", handleDeleteHost)
	mux.HandleFunc("POST /session/open", handleOpenSession)
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

	OpenBrowser("http://" + listen)

	wg.Wait()
}
