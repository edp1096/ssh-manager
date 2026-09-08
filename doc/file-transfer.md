# File transfer

Open a host's folder button to browse it over SFTP. The saved SSH username,
password or unencrypted private key is reused; `hosts.dat` is unchanged.
SSH terminal buttons continue to work independently.
Ctrl+Enter on a focused host row opens its SFTP file tab. Plain Enter still
opens the SSH terminal; Shift+Enter keeps its existing terminal split action.

The header keeps **Hosts** and connection tabs on the left and tools on the
right. Use **New FTP / FTPS connection** in the toolbar for FTP, explicit FTPS
(usually port 21), or implicit FTPS (usually port 990). These connection
settings are held in memory for the tab, not saved to disk. FTPS validates
the server certificate against the system trust store; plain FTP requires
confirmation because credentials and files are unencrypted.

Each connection tab has local and remote file lists. Enter a directory path,
double-click a folder, or use Up, Home and Refresh to navigate. On Windows, enter
another drive's absolute path to switch drives.

The file list starts with `.` (a decorative current-folder entry) and `..`
(double-click to open the parent folder). These rows stay first regardless of sorting and
can show the same selection highlight as files, but have no checkboxes and
cannot be transferred, renamed or deleted. At the filesystem root,
`..` remains visible but is disabled.

Select files or folders with the checkboxes, then use the context menu's Upload or Download. Files
go into the opposite pane's current directory. Each connection tab has its own
transfer history, counts and Clear finished action; other tabs are unaffected.
The Hosts tab has no transfer history. Switching tabs retains each tab's history.
Ctrl+W in an FTP/SFTP tab closes that connection tab using its close confirmation.
Ctrl+W on Hosts asks whether to exit the whole application; cancelling keeps it
open. Confirming requests shutdown of the application's managed browser window.
Tab/Shift+Tab focuses tab names, including inactive connections, and skips the
close buttons. Enter/Space activates a focused tab; Left/Right switches tabs.
Up/Down from a tab enters its list. Arrow keys from the toolbar, including New
FTP connection, return to the active host/file list. Close buttons still work
with the mouse; Ctrl+W closes the active connection tab.
The queue shows byte
progress, completion, errors and cancellation. Transfers run one at a time
in submission order and continue when switching tabs. Closing a connection
tab cancels its outstanding transfers after confirmation. Retry uses the
original destination as a new transfer, with the current connection's conflict
policy. Clear finished removes completed,
failed and cancelled jobs from the history without interrupting active jobs.

Single-click a file or folder row to select it. Ctrl+click toggles individual
entries; Shift+click selects a range in the displayed sort order. Clicking
empty list space clears selection. Checkboxes remain available; Ctrl+A selects
all entries and Escape clears selection. Double-click opens folders or transfers
the clicked file (upload from Local, download from Remote).

With focus in either file list, Up/Down moves the cursor in the displayed order
(including `.` and `..`). Shift+Up/Down
extends selection; Ctrl+Up/Down moves focus without changing selection.
Ctrl+Space toggles only the focused entry's checkbox, allowing non-contiguous
selection without the mouse. Left moves to Local and Right to Remote, restoring
each pane's cursor and preserving existing selections. Ctrl+Left/Right also
leaves all checkboxes unchanged. Path inputs retain normal text-editing keys.
PageUp/PageDown move the cursor by a visible page; Home/End move to the first/last
row. Ctrl preserves selection and Shift extends it, as with Up/Down.
Tab/Shift+Tab move between controls and lists, not each entry or checkbox.
Enter transfers all selected entries when multiple files/folders are selected,
even when the focused entry is a folder. Otherwise it opens the focused folder
or transfers the focused file: upload from Local, download from Remote.
Backspace opens the parent directory, except at a root.
F5 or Ctrl+R in an FTP/SFTP tab refreshes both lists at their current paths
without reloading the application or changing other tabs.
Folder navigation keeps focus in the list and returning to a parent restores
the cursor to the folder just left. Holding Enter or Backspace does not repeat
the action. Path-input editing and context-menu keys are handled separately.

Drag selected entries between the local and remote lists **within the same
tab** to enqueue a copy. Dropping on a folder uses that folder as the destination;
otherwise the current directory is used. The same conflict dialog and policy
apply. Dragging an already selected row preserves the multiple selection.
External desktop file drops, cross-tab transfers and same-side moves are not
supported. The `.` and `..` rows cannot be dragged or used as drop targets.

Right-click an entry for Open folder, Upload/Download, Rename, Delete, New folder
and Refresh. Right-clicking an existing selection preserves it; right-clicking
an unselected entry selects only that entry. Empty-space right-click shows the
folder-level actions. Escape or clicking elsewhere dismisses the menu.

Folder transfers include nested and empty directories. There is no permanent
overwrite checkbox or instruction bar. When an existing regular file conflicts,
an in-app modal shows incoming/existing paths, sizes and modification times.
Choose **Overwrite**, **Skip**, or **Cancel transfer**. Apply overwrite/skip to
this file only (default), this transfer (all entries submitted together), or
this connection tab (including later submissions). Connection policies are
in-memory and reset when the tab closes. Cancel stops the current submission,
including queued sibling entries, not unrelated transfers. Previously completed
files are not rolled back. Skipped files remain untouched and are counted in
the history without being counted as transferred bytes.

Conflict checks run before writing a job's files. While awaiting a decision the
remote connection is closed; continuing reconnects and scans again. A changed
destination's size/time invalidates a file-only decision. The sequential queue
waits behind the conflict; switch back to its connection tab to answer the dialog.
Directory contents are merged; files absent from the source are not removed.
Directory/file type conflicts, links and special files fail safely rather than
offering destructive replacement. Changes made by another writer after the
preflight check remain subject to the publication limitations below.

New folder, Rename, Delete and transfers are available in the context menu;
F2 renames a single selected file or folder. Rename uses an in-page text input
with IME composition support; Enter during composition does not submit the name.
there is no bottom action-button row. Delete requires
confirmation whether triggered by the context menu or Delete key
in the file list. Holding the Delete key does not repeat deletion prompts.
Confirmed deletion permanently removes selected folders including their contents.
Links themselves are removed without traversing their targets. Filesystem roots
and symlink ancestor paths are blocked. Deletion scans up to 128 levels and
100,000 entries before removing anything; an error or cancellation during removal
can leave a partially deleted folder. This operation does not use the trash.
Rename refuses a known existing destination. Click anywhere in the Name, Size or
Timestamp header cell to sort the listing. Folders remain before files.
Long names stay on one line with an ellipsis; hover to see the full name.
Modification times use the local timezone; unavailable times show `—`.

## Current limits

- No saved FTP profiles, resume or remote editor.
- Transfers are staged in sibling `.ssh-manager-*.part` files. Existing
  content is replaced only after streaming succeeds. Local overwrite and
  SFTP servers supporting the POSIX rename extension use atomic replacement.
  Other servers use a `.backup` file and attempt rollback on publication
  failure. Recovery paths are reported if cleanup or rollback fails.
- FTP does not provide a portable exclusive rename. Conflict checks cannot
  prevent another application from creating a destination between the check
  and rename. Avoid concurrent writers to the same destination. Local
  no-overwrite publication currently requires filesystem hard-link support.
- A failed folder transfer can leave already completed files and created
  directories. It is not a whole-folder transaction. A retry may therefore
  require resolving conflicts again on retry. Abrupt process termination can
  leave staging or backup files for manual recovery.
- Folder scans are limited to 128 levels and 100,000 entries per job. Progress
  starts after the scan; file contents are streamed, not buffered in memory.
- Permissions and timestamps are not preserved. Downloads are created with
  owner-only permissions on Unix. Symbolic links are not transferred.
- Tabs and queue history are in-memory only (up to 500 history entries).
- Each listing, operation or transfer uses an independent remote connection, so a
  transfer does not block navigation or affect external terminals. Servers
  must allow concurrent connections. Idle network operations time out after
  30 seconds; TCP connection attempts time out after 10 seconds.

## Implementation and checks

All confirmations, notices and text prompts use in-app dialogs rather than
browser alert/confirm/prompt windows. Inputs preserve IME composition; Enter
while composing does not submit. Cancel and Escape never authorize the action.
The startup host-file unlock dialog remains non-cancellable. Host/group forms,
password dialogs, FTP connection and file-operation dialogs share compact
spacing, aligned fields and right-aligned action buttons.

SFTP uses [`pkg/sftp`](https://github.com/pkg/sftp) v1.13.11 with the existing
`golang.org/x/crypto/ssh` dependency. It provides the required client operations
without CGO or an external SFTP executable. The v2 prerelease is not used.
FTP/FTPS uses [`jlaffaye/ftp`](https://github.com/jlaffaye/ftp) v0.2.4. Passive
data connections are used, with TLS on both control and data connections for
FTPS. Both libraries build without CGO. Server-specific FTP path formats and
TLS session-reuse requirements still need real-server compatibility testing.

Run `go test -race ./internal/filetransfer` for isolated SSH/SFTP integration
tests: SFTP password/key authentication, FTP and both FTPS modes, untrusted
certificate rejection, file/folder round trips, conflicts, cancellation,
overwrite preservation, rollback, symlink rejection and queue ordering.

Run `node tools/test-sftp-ui.mjs` from the repository root for a headless Chrome
test using the real file API and temporary files. It covers SFTP and FTP tabs,
file/folder transfers, overwrite, mkdir, rename and delete. Requires Node 22+, Go and
`google-chrome` (or set `CHROME`). The test also uses actual browser mouse events
for selection, double-click navigation, context menus and multi-file dragging.
It does not launch the application's browser
profile manager or access saved host credentials.

Linux browser testing and SFTP package cross-compilation are not Windows or
FreeBSD desktop validation, nor compatibility testing against every SFTP server.
