# Batch connections and connection groups

## Temporary selection

- Check hosts in **Hosts**. **Space** toggles the focused host; arrows and Enter retain their previous navigation/individual SSH behavior. **Ctrl+Enter** still opens SFTP.
- **Select all** includes hosts in collapsed categories. **Clear** clears the temporary selection.
- Click **Open in panels**, review the hosts and choose a split layout, then **Connect**.
- The first host opens a new terminal window. Remaining hosts become split panels in that window. Each batch gets its own window.
- **Alternating splits** alternates left/right and top/bottom, splitting the current/latest panel. It is not an equal-sized grid. Available screen space limits the number of useful panels; select fewer hosts if a terminal refuses another split.
- Up to 32 hosts may be requested per batch. Windows command-line length may impose a smaller limit.
- Existing individual connection buttons and authentication logic are unchanged. These new connections are not automatically added to any broadcast-input selection.

On Linux/FreeBSD, each terminal helper must acknowledge process startup before the next split is attempted. A failed split stops the batch; already opened terminals remain open. If the batch window has closed, later hosts are not redirected to another window.

Windows Terminal receives one command sequence targeting a fresh random window name, not the current window. A successful launcher exit means the requests were submitted; it does not prove every pane or SSH login succeeded. Inspect any opened panes before retrying a reported failure. No automatic retry is performed.

The Windows launch uses the documented [window targeting and split-pane command sequence](https://learn.microsoft.com/en-us/windows/terminal/command-line-arguments), without global terminal preference changes.

Terminal startup acknowledgement is not authentication confirmation on any platform. Check the login result inside each terminal.

## Saved connection groups

1. Select hosts, then **Save selection** and give the combination a name.
2. **Connection groups** toggles a sidebar on the left of Hosts. On narrow windows it overlays part of the list. The pin saves its open state across restarts. While pinned, the close button and toolbar toggle are disabled, and Escape cannot hide the panel. Unpin to enable hiding it again.
3. Click a group name to select its hosts and expand/collapse its numbered host list. Drag hosts to reorder them, or focus a host and press **Alt+Up/Down**. Changes save immediately; plain Up/Down moves focus without reordering. Up/Down and Home/End on group names move between groups.
4. Open **Split** to choose **Alternating splits**, **Left / right**, **Top / bottom**, or a grid. Hover the grid to preview numbered host positions and tooltips, then click to save. Arrow keys move the preview, Enter selects, and Escape cancels. Hovering alone never saves. **Connect** launches directly with the saved layout, without a review modal. Progress and errors appear at the bottom of the sidebar. Temporary **Open in panels** selections still use the review dialog and the same picker.
5. Use **⋯** or right-click a group for **Rename**, **Replace hosts**, or **Delete**. Replace hosts applies the current Hosts selection after confirmation: surviving members retain their group order and new hosts are appended in Hosts order. Delete removes only the combination, not the hosts.

Connection groups are independent of existing Hosts categories. Membership uses stable host IDs, not category/row indexes. Host renaming and reordering do not redirect saved connections. Groups preserve their saved connection order. Deleted hosts are shown as missing and block the group's Connect action until its membership is updated.

The panel header's side button moves the panel between left and right, including while pinned. Its position is saved as `panel-side` in `window.json` beside the executable and restored on startup; older settings default to the left. This does not change the pin setting stored with the connection groups.

Groups are stored in `hosts.dat.groups.json` beside the corresponding host file (or `<custom-host-file>.groups.json`). Only group names, group IDs, host IDs and split layouts are stored; passwords and private keys are not copied there. Existing groups without a layout use alternating splits and gain the layout field on the next save. The existing encrypted host-file format is unchanged. Save both files when backing up host data and its connection groups. Malformed group files are reported, not silently replaced.

## Grid layouts

Grid assigns hosts left to right, then top to bottom, using the group's saved order. Row count is automatic. The picker offers **Expand → Horizontally / Vertically**, with a numbered arrangement preview. Horizontal expansion shares each row's width among its hosts (the existing default). Vertical expansion shares each column's height among its hosts. With three hosts and two columns, horizontal expansion makes host 3 span the bottom; vertical expansion makes host 2 span the right side. No extra SSH connections are created.

The `grid-fill` preference is saved separately for each group; missing values retain horizontal expansion. In **Open in panels**, the choice applies only to that batch. With more than two rows, shorter columns distribute their height equally, so their internal boundaries may not align with adjacent taller columns.

The picker supports 1–32 columns. Only the minimum row count that fits all group members for a chosen column count can be selected; insufficient capacity and entirely empty extra rows are unavailable. Unused positions at the end of the last row are dimmed. These are placeholders in the picker, not actual empty terminals. Host reordering is independent of the main Hosts list and is persisted in the existing `host-ids` array.

Horizontal expansion uses equal-height rows and equal-width cells within each row; vertical expansion uses equal-width columns and equal-height cells within each column, subject to terminal minimum sizes, window space and character-cell rounding. The launcher creates row or column anchors first, so the **connection startup order** can differ from the final visual order. Failure messages identify the host whose launch failed; earlier opened connections remain open.

- **Tilix:** loads a generated private session layout with explicit split ratios. Each leaf has its own private control socket and expected terminal UUID, so asynchronous startup cannot interchange hosts. The layout contains helper commands, not credentials, and native synchronized input is disabled. Existing user terminal preferences are not modified.
- **Konsole:** this implementation requires **24.02.0 or later**. The split-control API was added in [commit f7732e3](https://github.com/KDE/konsole/commit/f7732e33c40e7787a83da53746af06326e0d34dc). It is absent from the [23.08.5 header](https://github.com/KDE/konsole/blob/v23.08.5/src/ViewManager.h) and present in the [24.02.0 header](https://github.com/KDE/konsole/blob/v24.02.0/src/ViewManager.h). This minimum applies to our verified-order grid control, not to ordinary/manual splitting. The launcher reads the hierarchy, corrects same-axis insertion order using `moveView`, reads it back, and applies `equal-size-view`. Older versions are rejected before launching a grid; existing non-grid layouts are unaffected.
- **Windows Terminal:** emits `move-focus first/nextInOrder` and `split-pane --size` commands in a single dedicated-window invocation. Requires those documented CLI commands. Terminal startup/focus/size behavior still needs a Windows desktop test; a successful launcher exit is not pane or authentication confirmation.

Tilix 2×2, 3×2 and 3×3 grids have been tested against an isolated SSH server, including every assigned login and near-equal PTY dimensions. Konsole and Windows Terminal remain unverified on a real desktop here.

## Launch snapshots

Each batch resolves all selected IDs before starting. It creates a temporary, private, encrypted host-file snapshot so edits or reordering during launch cannot change the target or credentials of later panels. These snapshots are removed on normal app exit. An abrupt process/OS termination can leave encrypted snapshots under the system temporary directory (`ssh-manager-batch-*`). They are not saved connection groups.

Batch launch operations and individual app launch requests are serialized. Later input-relay registration tokens in a batch remain pending for up to ten minutes, accommodating sequential terminal startup. Tokens are still single-use. Neither snapshotting nor batching changes key/password authentication inside `ssh-client`.

## Verification

- Linux/Tilix: production launcher tested with separate 3-pane and 2-pane batches against a disposable local SSH server; window isolation, alternating splits and broadcast-off behavior verified.
- Automated tests: ordered launch, stopping after failure, stable IDs, encrypted snapshot contents, group persistence/rename/update/delete, missing/duplicate IDs and corrupt-file preservation.
- Browser tests: checkbox/Space selection, selection across reloads, batch review, saved groups across Hosts reorder, missing-host blocking, membership replacement, rename/delete and existing file-browser behavior.
- Windows Terminal and Konsole: **desktop operation is not verified** here. Cross-builds and command/logic tests do not replace those platform tests.

```sh
go test -race ./...
node tools/test-sftp-ui.mjs
```

The opt-in `TestLiveTilixBroadcast` test also exercises the production batch launcher. See [broadcast.md](broadcast.md) for the isolated live-test setup.
