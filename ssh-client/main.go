package main // import "ssh-client"

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ssh-client/paneexit"
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
	hostsFile          = flag.String("f", "", "host data file (required)")
	hostFileKey        = flag.String("k", "", "host data file key which is base64 encoded (required)")
	hostIDX            = flag.Int("hi", 0, "index of host (required)")
	categoryIDX        = flag.Int("ci", 0, "index of category (required)")
	relayAddress       = flag.String("relay-address", "", "app input relay address")
	relayToken         = flag.String("relay-token", "", "one-use app input relay token")
	launchReadyAddress = flag.String("launch-ready-address", "", "app pane startup acknowledgement address")
	launchReadyToken   = flag.String("launch-ready-token", "", "one-use pane startup token")
	launchExitGroup    = flag.String("launch-exit-group", "", "Windows pane exit coordination group")

	// hosts []HostInfo
	hosts HostList
	host  HostInfo
	key   []byte
)

func main() {
	var err error

	flag.Parse()
	guard, err := paneexit.New(*launchExitGroup)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Pane exit coordination unavailable:", err)
	}
	defer func() {
		if err := guard.BeforeProcessExit(); err != nil {
			fmt.Fprintln(os.Stderr, "Pane exit coordination failed:", err)
		}
	}()
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
	if err = notifyLaunchReady(*launchReadyAddress, *launchReadyToken); err != nil {
		fmt.Println("Pane startup acknowledgement failed:", err)
		return
	}

	err = openSession()
	if err != nil {
		fmt.Println(err)
	}
}
