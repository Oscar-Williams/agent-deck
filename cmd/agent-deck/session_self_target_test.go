package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Self-target handoff (#2897): a command that targets the session it runs
// inside delegates to a detached copy of this binary instead of dying with
// the pane at the switch's stop. The parent exits 0 with a handoff receipt;
// the child completes the command from outside the pane.
//
// The subprocess tests run the real CLI with AGENTDECK_INSTANCE_ID pointing
// at the target, which is exactly the env a hook or a Bash tool call inside
// the session sees.

func pollSessionAccount(t *testing.T, home, id, want string) {
	t.Helper()
	var stdout, stderr string
	var code int
	deadline := time.Now().Add(20 * time.Second)
	for {
		stdout, stderr, code = runAgentDeck(t, home, "session", "show", id, "--json")
		if code == 0 {
			var resp struct {
				Account string `json:"account"`
			}
			if err := json.Unmarshal([]byte(stdout), &resp); err == nil && resp.Account == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("session %s account never became %q\nlast show: exit %d\nstdout: %s\nstderr: %s", id, want, code, stdout, stderr)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func TestSessionSwitchAccount_SelfTargetHandsOffAndChildCompletes(t *testing.T) {
	home := intendedSwitchHome(t)
	id := addTestSession(t, home, filepath.Join(home, "proj"), "worker")
	stdout, stderr, code := runAgentDeckEnv(t, home, "", []string{"AGENTDECK_INSTANCE_ID=" + id},
		"session", "switch-account", id, "work")
	if code != 0 {
		t.Fatalf("switch-account self-target failed (exit %d)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "detached") {
		t.Fatalf("no handoff receipt on stdout:\n%s", stdout)
	}
	// The detached child completes the switch: the account write lands even
	// though the parent that asked for it is long gone. (It also proves the
	// child did not delegate again — a loop would never set the account.)
	pollSessionAccount(t, home, id, "work")
}

func TestSessionSwitchAccount_InsideAnotherSessionRunsInPlace(t *testing.T) {
	home := intendedSwitchHome(t)
	id := addTestSession(t, home, filepath.Join(home, "proj"), "worker")
	stdout, stderr, code := runAgentDeckEnv(t, home, "", []string{"AGENTDECK_INSTANCE_ID=some-other-session"},
		"session", "switch-account", id, "work")
	if code != 0 {
		t.Fatalf("switch-account from another session failed (exit %d)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.Contains(stdout, "detached") {
		t.Fatalf("unexpected handoff for a non-self target:\n%s", stdout)
	}
	// The in-place path switched synchronously: no polling needed.
	stdout, stderr, code = runAgentDeck(t, home, "session", "show", id, "--json")
	if code != 0 {
		t.Fatalf("session show failed (exit %d)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var resp struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil || resp.Account != "work" {
		t.Fatalf("account = %q (err %v), want work\nstdout: %s", resp.Account, err, stdout)
	}
}

func TestSessionRestart_SelfTargetHandsOff(t *testing.T) {
	home := intendedSwitchHome(t)
	id := addTestSession(t, home, filepath.Join(home, "proj"), "worker")
	stdout, stderr, code := runAgentDeckEnv(t, home, "", []string{"AGENTDECK_INSTANCE_ID=" + id},
		"session", "restart", id)
	if code != 0 {
		t.Fatalf("restart self-target failed (exit %d)\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "detached") {
		t.Fatalf("no handoff receipt on stdout:\n%s", stdout)
	}
}
