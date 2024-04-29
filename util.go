package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v3/process"
)

func checkProcessExists(name string) (bool, error) {
	var err error
	var result bool = false

	procs, err := process.Processes()
	if err != nil {
		return result, err
	}
	for _, p := range procs {
		n, err := p.Name()
		if err != nil {
			continue
		}

		log.Println(n, name)

		// TODO: catch "tmux: server <nil>"
		if n == name {
			result = true
			break
		}
	}
	return result, nil
}

func getBinaryPath() (binPath, binName string, err error) {
	fullPath, err := os.Executable()
	if err != nil {
		return "", "", err
	}

	binPath = filepath.Dir(fullPath)
	binName = filepath.Base(fullPath)

	return binPath, binName, err
}

// func getCurrentPath() (cwd string, err error) {
// 	cwd, err = os.Getwd()
// 	if err != nil {
// 		return "", err
// 	}

// 	return cwd, err
// }
