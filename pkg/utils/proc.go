package utils

import "github.com/shirou/gopsutil/v3/process"

func CheckProcessExists(name string) (bool, error) {
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

		// TODO: catch "tmux: server <nil>"
		if n == name {
			result = true
			break
		}
	}
	return result, nil
}
