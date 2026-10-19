# bubbletea v1.3.10, patched

This is github.com/charmbracelet/bubbletea v1.3.10 (go.sum h1 of the module: see the repo's go.sum history), without its `_test.go` files, plus one change:

- `tty.go`, `initCancelReader`: close the input reader being replaced once its read loop has stopped (#2692). Without it every `RestoreTerminal` (every `tea.Exec`, i.e. every attach) leaks the old reader's kqueue (macOS) or epoll (Linux) descriptor.

Check: `diff -r "$(go env GOMODCACHE)/github.com/charmbracelet/bubbletea@v1.3.10" third_party/bubbletea` shows only that hunk, this file, and the removed tests.

Drop this directory and the `replace` in go.mod once upstream closes the replaced reader.
