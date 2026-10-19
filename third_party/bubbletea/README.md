# bubbletea v1.3.10 (patched copy)

Upstream: github.com/charmbracelet/bubbletea **v1.3.10**, the module as `go mod download` fetches it and go.sum verifies it, with its `_test.go` files and repository files (README, CI config, Taskfile) left out. LICENSE is upstream's (MIT).

agent-deck uses it through `replace github.com/charmbracelet/bubbletea => ./third_party/bubbletea` in go.mod.

## The one change

`tty.go`, in `initCancelReader`, before the new reader is created:

```go
	// agent-deck patch (#2692): close the reader being replaced. ...
	if p.cancelReader != nil && p.readLoopDone != nil {
		select {
		case <-p.readLoopDone:
			_ = p.cancelReader.Close()
		default:
		}
	}
```

Without it every `RestoreTerminal` (every `tea.Exec`: each attach, the editor, the updater) leaks the replaced reader's kqueue (macOS) or epoll (Linux) descriptor.

## Drift check

`internal/thirdparty/bubbletea_copy_test.go` pins the SHA-256 of every upstream file. It fails if a file is added, removed or changed, or if `tty.go` differs from upstream by anything but the hunk above. To check by hand:

    diff -r "$(go env GOMODCACHE)/github.com/charmbracelet/bubbletea@v1.3.10" third_party/bubbletea

## Removal

Drop this directory and the `replace` once an upstream release closes the replaced reader.
