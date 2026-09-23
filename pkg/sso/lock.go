package sso

import (
	"fmt"
	"os"
	"path/filepath"
)

// AcquireAuthorizationLock serializes authentication and forced cache invalidation
// for this user. The OS releases the lock even on os.Exit or process termination.
func AcquireAuthorizationLock() (func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return acquireAuthorizationLockAt(filepath.Join(home, ".aws", "sso", "cache", "go-aws-sso-auth.lock"))
}

func acquireAuthorizationLockAt(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create authorization lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open authorization lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	// Keep the file in place: unlinking lets another process lock a different
	// inode while the original lock is still held. File contents/age are irrelevant.
	return func() { _ = f.Close() }, nil
}
