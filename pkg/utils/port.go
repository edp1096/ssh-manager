package utils

import (
	"fmt"
	"net"
	"time"

	mrand "math/rand"
)

func GetAvailablePort() (port int, err error) {
	portBegin := 10000
	portEnd := 50000

	source := mrand.NewSource(time.Now().UnixNano())
	randGen := mrand.New(source)

	for {
		port = randGen.Intn(portEnd-portBegin) + portBegin
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			break
		}
	}

	return
}
