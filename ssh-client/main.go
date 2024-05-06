package main // import "ssh-client"

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type HostList struct {
	Categories []HostCategory `json:"host-categories"`
}

type HostCategory struct {
	Name  string     `json:"name"`
	Hosts []HostInfo `json:"hosts"`
}

type HostInfo struct {
	Name           string
	Description    string
	Address        string
	Port           int
	Username       string
	Password       string
	PrivateKeyText string
}

var (
	hostsFile   = flag.String("f", "", "host data file (required)")
	hostFileKey = flag.String("k", "", "host data file key which is base64 encoded (required)")
	hostIDX     = flag.Int("hi", 0, "index of host (required)")
	categoryIDX = flag.Int("ci", 0, "index of category (required)")

	// hosts []HostInfo
	hosts HostList
	host  HostInfo
	key   []byte
)

func main() {
	var err error

	flag.Parse()
	if flag.NArg() > 0 || *hostsFile == "" || *hostFileKey == "" || *categoryIDX == 0 || *hostIDX == 0 {
		binaryName := filepath.Base(os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", binaryName)
		flag.PrintDefaults()
		return
	}

	// key = []byte("0123456789!#$%^&*()abcdefghijklm")
	// key, err = generateKey(*hostFileKey)
	// if err != nil {
	// 	fmt.Println("error key generation")
	// 	return
	// }
	key, _ = base64.URLEncoding.DecodeString(*hostFileKey)

	err = loadHostData(*hostsFile, key, &hosts)
	if err != nil {
		fmt.Println("error loading host data file")
		return
	}

	*categoryIDX--
	*hostIDX--

	if *categoryIDX > len(hosts.Categories)-1 {
		fmt.Printf("category index not exist. max index is %d\n", len(hosts.Categories))
		return
	}

	if *hostIDX > len(hosts.Categories[*categoryIDX].Hosts)-1 {
		fmt.Printf("host index not exist. max index is %d\n", len(hosts.Categories))
		return
	}

	host = hosts.Categories[*categoryIDX].Hosts[*hostIDX]
	fmt.Printf("Connecting %s/%s\n", host.Name, host.Address)

	err = openSession()
	if err != nil {
		fmt.Println(err)
	}
}
