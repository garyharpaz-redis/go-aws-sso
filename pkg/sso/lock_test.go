package sso

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthorizationLockLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "auth.lock")
	release, err := acquireAuthorizationLockAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// An old timestamp cannot make a live lock expire.
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if unlock, err := acquireAuthorizationLockAt(path); err == nil {
		unlock()
		t.Fatal("acquired an already-held lock")
	}
	release()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file must remain in place: %v", err)
	}
	releaseAgain, err := acquireAuthorizationLockAt(path)
	if err != nil {
		t.Fatalf("could not reacquire released lock: %v", err)
	}
	releaseAgain()
}

func TestAuthorizationLockReleasedOnProcessExit(t *testing.T) {
	for _, mode := range []string{"fatal", "killed"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "auth.lock")
			cmd := exec.Command(os.Args[0], "-test.run=^TestAuthorizationLockHelper$")
			cmd.Env = append(os.Environ(), "GO_AWS_SSO_LOCK_TEST_PATH="+path, "GO_AWS_SSO_LOCK_TEST_MODE="+mode)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			ready := make(chan string, 1)
			go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
			select {
			case line := <-ready:
				if strings.TrimSpace(line) != "locked" {
					t.Fatalf("child failed to lock: %q", line)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("child did not become ready")
			}
			if release, err := acquireAuthorizationLockAt(path); err == nil {
				release()
				t.Fatal("another process acquired a live lock")
			}
			if mode == "killed" {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := stdin.Write([]byte("exit\n")); err != nil {
					t.Fatal(err)
				}
			}
			if err := cmd.Wait(); err == nil {
				t.Fatal("expected unsuccessful child exit")
			}
			release, err := acquireAuthorizationLockAt(path)
			if err != nil {
				t.Fatalf("process exit left a stale lock: %v", err)
			}
			release()
		})
	}
}

func TestAuthorizationLockHelper(t *testing.T) {
	path := os.Getenv("GO_AWS_SSO_LOCK_TEST_PATH")
	if path == "" {
		return
	}
	release, err := acquireAuthorizationLockAt(path)
	if err != nil {
		os.Exit(2)
	}
	defer release()
	os.Stdout.WriteString("locked\n")
	bufio.NewReader(os.Stdin).ReadString('\n')
	// Like zap.Fatal, os.Exit skips deferred cleanup.
	os.Exit(1)
}
