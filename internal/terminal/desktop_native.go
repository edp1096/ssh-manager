//go:build linux || freebsd

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Desktop names describe the login session, not the installed GTK/Qt libraries.
func selectBackend(getenv func(string) string, lookup func(string) (string, error)) (string, error) {
	if override := strings.ToLower(strings.TrimSpace(getenv("SSH_MANAGER_TERMINAL"))); override != "" {
		if override != "tilix" && override != "konsole" {
			return "", fmt.Errorf("SSH_MANAGER_TERMINAL에는 tilix 또는 konsole을 지정하세요.")
		}
		return override, nil
	}
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "DESKTOP_SESSION"} {
		for _, name := range strings.FieldsFunc(strings.ToLower(getenv(key)), func(r rune) bool { return r == ':' || r == ';' }) {
			switch strings.TrimSpace(name) {
			case "kde", "plasma", "plasmawayland", "plasmax11":
				return "konsole", nil
			case "gnome", "ubuntu", "ubuntu-wayland", "gnome-classic", "unity", "xfce", "xfce4", "x-cinnamon", "cinnamon", "mate", "budgie", "pantheon":
				return "tilix", nil
			}
		}
	}
	for _, name := range []string{"tilix", "konsole"} {
		if _, err := lookup(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("데스크톱 환경을 확인할 수 없습니다. Tilix 또는 Konsole을 설치하고 SSH_MANAGER_TERMINAL=tilix 또는 konsole을 지정하세요.")
}

func terminalStatus(getenv func(string) string, lookup func(string) (string, error), osRelease string) TerminalStatus {
	backend, err := selectBackend(getenv, lookup)
	s := TerminalStatus{Backend: backend}
	if err != nil {
		s.Message = err.Error()
		return s
	}
	if _, err = lookup(backend); err != nil {
		s.Message = fmt.Sprintf("%s가 설치되어 있지 않거나 PATH에서 찾을 수 없습니다. 로컬 화면 분할에 %s가 필요합니다.", backend, backend)
		switch {
		case strings.Contains(strings.ToLower(osRelease), "freebsd"):
			s.Message += "\n설치: pkg install " + backend
		case strings.Contains(osRelease, "debian"), strings.Contains(osRelease, "ubuntu"):
			s.Message += "\n설치: sudo apt install " + backend
		case strings.Contains(osRelease, "fedora"):
			s.Message += "\n설치: sudo dnf install " + backend
		case strings.Contains(osRelease, "arch"):
			s.Message += "\n설치: sudo pacman -S " + backend
		}
		s.Message += "\n설치 후 다시 연결하세요. 앱을 재시작할 필요는 없습니다."
		return s
	}
	if backend == "konsole" {
		if _, err = lookup("gdbus"); err != nil {
			s.Message = "Konsole 분할 제어에 gdbus가 필요합니다. 배포판의 GLib 도구 패키지를 설치하세요."
			if strings.Contains(strings.ToLower(osRelease), "freebsd") {
				s.Message += "\n설치: pkg install glib"
			}
			return s
		}
	}
	s.Ready = true
	return s
}

func Status() TerminalStatus {
	if runtime.GOOS == "freebsd" {
		return terminalStatus(os.Getenv, exec.LookPath, "freebsd")
	}
	release, _ := os.ReadFile("/etc/os-release")
	return terminalStatus(os.Getenv, exec.LookPath, string(release))
}

func CheckTerminalExist() {
	if s := Status(); !s.Ready {
		fmt.Println(s.Message)
	}
}
