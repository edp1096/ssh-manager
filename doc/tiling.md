# Terminal tiling

On Linux and FreeBSD, terminal splitting uses Tilix or Konsole without tmux.

## Terminal selection

| Desktop | Default terminal |
| --- | --- |
| GNOME / Ubuntu | Tilix |
| KDE Plasma | Konsole |
| Unrecognized | Installed terminal, preferring Tilix over Konsole |

The current desktop session determines the default. Installing additional GTK or Qt libraries does not change the selection. To choose a terminal explicitly:

```sh
SSH_MANAGER_TERMINAL=tilix ./bin/ssh-manager
SSH_MANAGER_TERMINAL=konsole ./bin/ssh-manager
```

If the selected terminal is missing, the app displays installation instructions. On Ubuntu, install it with `sudo apt install tilix` or `sudo apt install konsole libglib2.0-bin`.

On FreeBSD, run `pkg install tilix` for GNOME or `pkg install konsole glib` for KDE as root. The terminal and `gdbus` must be on PATH (normally `/usr/local/bin`). Run the app inside the graphical desktop session; Konsole control uses its session D-Bus.

FreeBSD port definitions: [Tilix](https://github.com/freebsd/freebsd-ports/tree/main/x11/tilix), [Konsole](https://github.com/freebsd/freebsd-ports/tree/main/x11/konsole), [GLib including gdbus](https://github.com/freebsd/freebsd-ports/blob/main/devel/glib20/pkg-plist). Package availability depends on the FreeBSD release, architecture, and configured repository.

## Split behavior

| Action | Result |
| --- | --- |
| First connection | Open a new terminal window |
| Normal connection | Add a pane on the right |
| Vertical panel | Add a pane below |
| New window | Open a separate window and prefer it for subsequent connections |

The split target is the **most recently connected, still-running pane** in a window opened by the app. If that pane closes, the previous running pane is used. If the entire window closes, the app uses another window it opened or creates a new one. Changing mouse focus does not change the target.

Use SSH Manager's buttons to create splits. Currently, a pane created manually through Konsole's menus or shortcuts may close because it receives no connection request.

## Integration

- **Tilix:** Targets panes using a separate group for each window and a terminal UUID. Tilix marks the group option as experimental.
- **Konsole:** Runs in a separate process and uses D-Bus to select and split the target session.

A helper in each new pane receives and runs the SSH command from the app. Commands are not typed into existing SSH sessions. Launch and split failures are displayed in the app.

Windows continues to use Windows Terminal. The `hosts.dat` format remains unchanged.

Both the connection-group Split picker and the Open in panels Split picker offer `Expand: Vertically (left)`. This right-aligns the incomplete final row: with three hosts and two columns, host 1 spans the left side while hosts 2 and 3 occupy the upper and lower right. Existing `Vertically` keeps hosts 1 and 3 on the left and host 2 on the right. Connection groups persist this option as `grid-fill: vertical-left`.

Connection groups can be reordered by dragging their names, pressing Alt+Up/Down on a focused group name, or choosing Move up/down in the group menu. The group list order is saved beside the host file without changing host order within groups or their split settings.

Windows batches launch one pane at a time and wait for a one-use localhost acknowledgement from `ssh-client` before issuing the next command. This avoids the Windows Terminal 1.24 startup hang reproduced with a three-host, two-column vertical-fill grid. Grid targets use pane creation IDs, not traversal order. Rebuild and distribute both executables together. If a pane does not acknowledge startup within 15 seconds, the batch stops and reports the number confirmed started.

Windows live grid checks use local probe processes rather than SSH connections:

The probe checks three-host/two-column and six-host/three-column layouts. It also verifies that the native test window disappears after the probe processes exit. Test panes normally exit one at a time in reverse creation order. Set `SSH_MANAGER_WT_CLOSE_ORDER=forward` or `simultaneous` to test other exit sequences.

On Windows Terminal 1.24.11911.0, simultaneous exits in a three-host vertical grid left an orphaned window even though every process exited successfully. The window still responded to native messages; this was not an SSH process hang. An A/B test with animations temporarily disabled isolated the animation-related behavior. The original settings were restored byte-for-byte and are not changed by SSH Manager.

Windows batches of three or more clients now coordinate normal process exits with a per-window named mutex. A client retains ownership until its process exits; the next client detects the abandoned mutex and waits one second for the pane animation before exiting. The mutex is independent of the manager's lifetime, and separate batch windows do not block each other. Single/two-client launches and Linux/FreeBSD do not use this delay. This does not intercept forced process termination. With the original animations enabled, simultaneous-exit live tests passed for three-host horizontal, vertical and left-expanded grids, and six-host horizontal and vertical grids, including native-window closure. Rebuild both the manager and SSH client together.

```powershell
$env:SSH_MANAGER_WT_LIVE = "$env:LOCALAPPDATA/Microsoft/WindowsApps/wt.exe"
go test ./internal/terminal -run TestLiveWindowsGrid -v -count=1
```

## Verification status

| Target | Result |
| --- | --- |
| Tilix 1.9.4 | Live tests passed for window creation, side-by-side and stacked splits, separate window groups, and pane closure |
| Konsole 26.08.0 | Launch options and split interfaces checked against the official source; live execution not yet verified |
| Automated tests | Passed for terminal selection, installation notices, target identification, communication, and error handling |
| FreeBSD | Shares the native backend and tests with Linux; actual FreeBSD GUI execution has not been verified |

The Tilix live test used short-lived local commands. Input, scrolling, and resizing during actual SSH connections still need verification.

To rerun the live test, use the commands below. Tilix windows briefly open and then close.

```sh
go build -o bin/ssh-manager .
SSH_MANAGER_LIVE_TEST_BINARY="$PWD/bin/ssh-manager" go test ./internal/terminal -run TestLiveTilix -v -count=1
```
