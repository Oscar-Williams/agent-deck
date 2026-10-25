package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// sessionSelfTargetChildEnv marks a process handed off by the self-target
// path below, so a follow-up that still runs inside the target pane can
// never delegate again by accident.
const sessionSelfTargetChildEnv = "AGENTDECK_SELF_TARGET_CHILD"

// runningInsideInstanceID reports the session this CLI process runs inside,
// as injected into the pane env by the start path (AGENTDECK_INSTANCE_ID,
// see cross_harness_switch.go's env source command). Empty outside tmux.
func runningInsideInstanceID() string {
	return strings.TrimSpace(os.Getenv("AGENTDECK_INSTANCE_ID"))
}

// selfTargeted reports whether this invocation targets the session it runs
// inside. That is the natural place to ask for a switch or a restart, and
// the one place those commands cannot run from: the switch's stop (and any
// restart) tears down the pane, which takes this process with it mid-run.
func selfTargeted(instID string) bool {
	id := runningInsideInstanceID()
	return id != "" && id == strings.TrimSpace(instID) &&
		os.Getenv(sessionSelfTargetChildEnv) == ""
}

// envWithoutInstanceID is the child's environment: the caller's verbatim,
// minus the pane's own session marker, plus the delegation marker. The
// follow-up re-runs the same command from outside the pane; it must see
// itself as not self-targeted.
func envWithoutInstanceID() []string {
	env := os.Environ() //nolint:forbidigo // verbatim pass-through to the detached follow-up, same as startBackgroundCommand
	kept := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, "AGENTDECK_INSTANCE_ID=") {
			continue
		}
		kept = append(kept, kv)
	}
	return append(kept, sessionSelfTargetChildEnv+"=1")
}

// replacedIdentifierArgs is childArgs with the identifier the caller typed
// replaced by the resolved instance ID, so the follow-up resolves the exact
// session even if the caller used a title that stops matching mid-switch.
func replacedIdentifierArgs(args []string, instID string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, instID)
	out = append(out, args[1:]...)
	return out
}

// delegateSelfTargetCommand hands the command to a detached copy of this
// binary and reports whether the handoff happened. The follow-up runs the
// normal path from outside the pane's process tree (Setsid, like
// startBackgroundCommand), so the teardown it performs cannot kill it, and
// its output lands in a log file named in the receipt.
//
// A false return means the handoff could not be set up; the caller falls
// through to the in-place command, which behaves as before this path
// existed (and, inside the target pane, still dies with the pane).
func delegateSelfTargetCommand(out *CLIOutput, profile string, subcommand string, args []string, instID string) bool {
	self, err := os.Executable()
	if err != nil {
		out.Error(fmt.Sprintf("resolving this binary for the detached follow-up: %v", err), ErrCodeInvalidOperation)
		return false
	}
	logFile, err := os.CreateTemp("", "agent-deck-self-target-*.log")
	if err != nil {
		out.Error(fmt.Sprintf("creating the follow-up log file: %v", err), ErrCodeInvalidOperation)
		return false
	}
	childArgs := append([]string{"session", subcommand}, replacedIdentifierArgs(args, instID)...)
	childArgs = profileArgs(profile, childArgs...)
	child := exec.Command(self, childArgs...) //nolint:gosec // G702: own binary, no shell
	child.Env = envWithoutInstanceID()
	child.Stdin, child.Stdout, child.Stderr = nil, logFile, logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		_ = logFile.Close()
		out.Error(fmt.Sprintf("starting the detached follow-up: %v", err), ErrCodeInvalidOperation)
		return false
	}
	_ = logFile.Close()
	_ = child.Process.Release()
	out.Success(fmt.Sprintf("Running inside the target session; handed %q to a detached process (log: %s). The pane teardown would otherwise kill the command mid-run.",
		strings.Join(childArgs, " "), filepath.ToSlash(logFile.Name())),
		map[string]interface{}{
			"handed_off": true,
			"log":        logFile.Name(),
			"child_args": childArgs,
		})
	return true
}
