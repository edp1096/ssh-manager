# Broadcast input

Broadcast input sends terminal input from one app-opened SSH connection to selected app-opened connections. It works inside `ssh-client`, independently of the terminal's native broadcasting features. It does not capture desktop keyboard events or control unrelated terminals.

## Use

1. Open two or more SSH connections from SSH Manager.
2. Click the keyboard / **OFF** button in the toolbar.
3. Select one **Source** and one or more **Receive** connections, then **Start**.
   The checkbox in the **Receive** header selects or clears all receivers except the source. A partial selection shows a mixed state. Select a source first; receiver selection is locked while broadcasting.
4. Type in the source terminal. Its input also reaches the selected receivers, including connections in other windows or panes.
5. Click **Stop**, or press **Ctrl+]** in any selected terminal.

Each connection has a separate ID, even when several connect to the same host. The ID appears in the connection list and in an initial `SSH connection` message in the terminal. A terminal title is also requested, but terminal settings and remote shells may replace it.

The toolbar shows **ON** while broadcasting. Closing the selection dialog does **not** stop it. Only the source broadcasts; typing in a receiver remains local to that receiver. Stop before changing the source or receivers. New connections are never selected automatically.

## Behavior and limits

- Normal characters, pasted text, Enter and terminal control sequences use the existing SSH input path. Received input is not rebroadcast.
- Output remains independent. Echo depends on the remote terminal/application; password input may be invisible.
- Passwords, control keys and destructive commands are also duplicated. Check that receivers are at the expected prompt before starting.
- Turn off Tilix/Konsole/Windows Terminal's own input synchronization to avoid duplicate input or bypassing the selected source/receivers.
- `Ctrl+]` is reserved while a connection is selected for broadcasting. An input chunk containing it is consumed as an emergency stop, not sent to SSH. With broadcast off it retains its normal behavior.
- Disconnecting a selected client stops the group. Ordinary socket closure is detected promptly; an unresponsive relay connection times out after about 12 seconds. Already delivered input cannot be recalled.
- Slow/full relay queues stop broadcasting rather than silently skipping input and continuing. This is not transactional execution: a command can reach one server before another disconnects.
- Closing the app stops its relay. If an SSH terminal remains open, its ordinary local input path continues without broadcasting. Restarting the app does not attach old connections automatically.
- Both `ssh-manager` and `ssh-client` must be updated together. Independently launched terminals and clients without an app-issued relay token do not appear.
- No host-file format, terminal preferences, IME settings or remote server configuration changes are required.

## Implementation

The manager creates a loopback-only TCP relay on demand. Each SSH launch receives a random, short-lived, single-use registration token. Registration happens after SSH authentication and shell startup. The browser receives connection labels/IDs, not tokens. Input is held only in bounded in-memory queues and is not logged or saved.

Source/receiver state has a generation number. Stale input cannot be replayed into a newly selected group. Terminal input and relayed input share a serialized SSH writer; only locally typed source input can enter the relay. Control connections have heartbeat and write deadlines. Terminal EOF closes the SSH session and relay registration.

## Verification

Verified on Linux with real Tilix windows and split panes, an isolated local SSH server and real `ssh-client` processes:

- one source, two receivers and one excluded pane across two windows;
- byte-for-byte delivery, arrow/control keys, emergency stop and reselection;
- receiver SSH disconnection and destruction of the source's test window;
- no input delivered to the excluded connection.

Automated broker/client tests also cover Unicode bytes, stale generations, token reuse, receiver non-rebroadcast and ordinary input after relay closure. Browser tests cover selection, state display, stopping, disconnected entries and modal layout.

Windows Terminal and Konsole desktop behavior is **not yet verified**. Windows/FreeBSD cross-builds only check compilation; they are not desktop tests. The production implementation adds relay arguments to the existing launch path and does not invoke either terminal's broadcast API.

Tests:

```sh
go test -race ./...
(cd ssh-client && go test -race ./...)
node tools/test-sftp-ui.mjs
```

Opt-in live Tilix test (opens disposable windows and sends test-only keystrokes):

```sh
go build -o bin/ssh-manager .
go build -o bin/ssh-client ./ssh-client
SSH_MANAGER_BROADCAST_LIVE_BINARY="$PWD/bin/ssh-manager" \
  go test -race ./internal/terminal -run '^TestLiveTilixBroadcast$' -v -count=1
```

The live input driver requires X11/XWayland Tilix, Python 3, libX11 and libXtst. It checks the exact test process and a visible window owned by it before typing. It never changes global terminal settings. The recording SSH server does not execute commands, and the test uses a temporary encrypted host file rather than the user's data.
