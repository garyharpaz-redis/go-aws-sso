//go:build unix

package sso

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return errors.New(lockedAuthFlowMsg)
	}
	if err != nil {
		return fmt.Errorf("lock authorization flow: %w", err)
	}
	return nil
}
