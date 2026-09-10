//go:build linux || freebsd

package terminal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func tilixUUID() string {
	h := strings.TrimPrefix(batchID(), "ssh-manager-batch-")
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func quoteHelperArg(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// Tilix loads a complete split tree. Leaves use independent private sockets:
// startup scheduling cannot swap the SSH arguments assigned to two panels.
func tilixGridLayout(args []SshClientArgument, exe, dir, profile string) (map[string]any, []paneIdentity) {
	ids := make([]paneIdentity, len(args))
	leaves := make([]map[string]any, len(args))
	for i := range args {
		ids[i] = paneIdentity{TilixID: tilixUUID()}
		command := quoteHelperArg(exe) + " " + childFlag + " " + quoteHelperArg(filepath.Join(dir, "pane-"+strconv.Itoa(i)))
		leaves[i] = map[string]any{"type": "Terminal", "uuid": ids[i].TilixID, "profile": profile, "directory": dir, "overrideCommand": command, "synchronizedInput": false, "readOnly": false}
	}
	// Rebuild the same tree the launch plan describes; each split replaces a leaf.
	root := leaves[0]
	for i := 1; i < len(args); i++ {
		target := leaves[args[i].GridTarget-1]
		old := map[string]any{}
		for k, v := range target {
			old[k] = v
			delete(target, k)
		}
		leaves[args[i].GridTarget-1] = old
		orientation := 0
		if args[i].SplitVertical {
			orientation = 1
		}
		target["type"] = "Paned"
		target["orientation"] = orientation
		target["ratio"] = 1 - args[i].SplitSize
		target["position"] = 1 - args[i].SplitSize
		target["child1"] = old
		target["child2"] = leaves[i]
	}
	return map[string]any{"version": "1.0", "type": "Session", "name": "SSH grid", "width": 1200, "height": 800, "synchronizedInput": false, "child": root}, ids
}

func openTilixGrid(args []SshClientArgument) (int, error) {
	nativeState.Lock()
	defer nativeState.Unlock()
	exe, err := terminalExecutable()
	if err != nil {
		return 0, err
	}
	client := filepath.Join(filepath.Dir(exe), "ssh-client")
	if _, err = exec.LookPath(client); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	profileBytes, err := exec.CommandContext(ctx, "gsettings", "get", "com.gexperts.Tilix.ProfilesList", "default").Output()
	profile := strings.Trim(strings.TrimSpace(string(profileBytes)), "'")
	if err != nil || !tilixIDPattern.MatchString(profile) {
		return 0, fmt.Errorf("cannot read Tilix default profile for grid layout")
	}
	w, err := newNativeWindow("tilix")
	if err != nil {
		return 0, err
	}
	w.batchID = batchID()
	controllers := make([]*nativeWindow, len(args))
	defer func() {
		for _, c := range controllers {
			if c != nil {
				c.listener.Close()
			}
		}
	}()
	for i := range args {
		l, e := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(w.dir, "pane-"+strconv.Itoa(i)), Net: "unix"})
		if e != nil {
			w.close()
			return 0, e
		}
		controllers[i] = &nativeWindow{backend: "tilix", listener: l}
	}
	layout, identities := tilixGridLayout(args, exe, w.dir, profile)
	data, err := json.Marshal(layout)
	if err != nil {
		w.close()
		return 0, err
	}
	file := filepath.Join(w.dir, "grid.json")
	if err = os.WriteFile(file, data, 0600); err != nil {
		w.close()
		return 0, err
	}
	cmd := exec.Command("tilix", "--group="+filepath.Base(w.dir), "--session="+file)
	cmd.Env = cleanTerminalEnv(os.Environ())
	if err = cmd.Start(); err != nil {
		w.close()
		return 0, err
	}
	go cmd.Wait()
	nativeState.windows = append(nativeState.windows, w)
	for i, arg := range args {
		file := arg.HostsFile
		if strings.HasPrefix(file, "./") {
			file = filepath.Join(filepath.Dir(exe), filepath.Base(file))
		}
		file, err = filepath.Abs(file)
		if err != nil {
			return i, err
		}
		argv := []string{client, "-f", file, "-k", base64.URLEncoding.EncodeToString(arg.HostFileKEY), "-ci", strconv.Itoa(arg.CategoryIndex), "-hi", strconv.Itoa(arg.HostIndex)}
		argv = append(argv, relayArguments(arg)...)
		if err = controllers[i].acceptExpectedPane(argv, nil, &identities[i]); err != nil {
			return i, fmt.Errorf("Tilix grid startup failed: %w", err)
		}
		w.panes = append(w.panes, controllers[i].panes...)
	}
	return len(args), nil
}
