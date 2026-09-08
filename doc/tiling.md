# Terminal tiling

On Linux, terminal splitting uses Tilix or Konsole.

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

The existing Windows and FreeBSD launch behavior and the `hosts.dat` format remain unchanged.

## Verification status

| Target | Result |
| --- | --- |
| Tilix 1.9.4 | Live tests passed for window creation, side-by-side and stacked splits, separate window groups, and pane closure |
| Konsole 26.08.0 | Launch options and split interfaces checked against the official source; live execution not yet verified |
| Automated tests | Passed for terminal selection, installation notices, target identification, communication, and error handling |

The Tilix live test used short-lived local commands. Input, scrolling, and resizing during actual SSH connections still need verification.

To rerun the live test, use the commands below. Tilix windows briefly open and then close.

```sh
go build -o bin/ssh-manager .
SSH_MANAGER_LIVE_TEST_BINARY="$PWD/bin/ssh-manager" go test ./internal/terminal -run TestLiveTilix -v -count=1
```
