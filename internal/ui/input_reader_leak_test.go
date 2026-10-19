package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
)

// A user's TUI held 2,822 kqueue descriptors after days of attaching and
// detaching (#2692). Every attach is a tea.Exec: Bubble Tea releases the
// terminal and, on return, restores it with a NEW input reader without closing
// the old one. On macOS each reader owns a kqueue, on Linux an epoll instance:
// a raw descriptor no finalizer closes (the reader's pipes are *os.File and
// are). This drives the TUI's own input setup through attach cycles and counts
// the poll descriptors this process holds.

type leakModel struct{ started chan struct{} }

func (m leakModel) Init() tea.Cmd                       { return func() tea.Msg { close(m.started); return nil } }
func (m leakModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m leakModel) View() string                        { return "" }

// pollDescriptors counts this process's epoll instances.
func pollDescriptors(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("read fds: %v", err)
	}
	n := 0
	for _, e := range entries {
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name())); err == nil && target == "anon_inode:[eventpoll]" {
			n++
		}
	}
	return n
}

func TestAttachCyclesDoNotLeakInputPollDescriptors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("counts epoll instances in /proc/self/fd")
	}
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()
	go func() { // drain the program's output
		buf := make([]byte, 4096)
		for {
			if _, err := ptmx.Read(buf); err != nil {
				return
			}
		}
	}()

	m := leakModel{started: make(chan struct{})}
	p := tea.NewProgram(m, tea.WithInput(NewCSIuReader(tty)), tea.WithOutput(tty), tea.WithoutSignalHandler())
	done := make(chan struct{})
	go func() { _, _ = p.Run(); close(done) }()
	select {
	case <-m.started:
	case <-time.After(10 * time.Second):
		t.Fatal("program did not start")
	}

	before := pollDescriptors(t)
	const attaches = 20
	for i := 0; i < attaches; i++ { // what every tea.Exec (attach) does
		if err := p.ReleaseTerminal(); err != nil {
			t.Fatalf("release: %v", err)
		}
		if err := p.RestoreTerminal(); err != nil {
			t.Fatalf("restore: %v", err)
		}
	}
	runtime.GC()
	after := pollDescriptors(t)
	p.Quit()
	<-done
	if after-before > 1 {
		t.Fatalf("%d attach cycles left %d more poll descriptors open (%d -> %d): one leaked per attach", attaches, after-before, before, after)
	}
}
