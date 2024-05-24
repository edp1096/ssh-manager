package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func GetBinaryPath() (binPath, binName string, err error) {
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

func CheckFileExitsInEnvPath(fname string) (result bool) {
	paths := strings.Split(os.Getenv("PATH"), ":")

	result = false
	for _, p := range paths {
		if _, err := os.Stat(p + "/" + fname); err == nil {
			// fmt.Printf("%s exists in %s\n", fname, p)
			result = true
			break
		}
	}

	return result
}
