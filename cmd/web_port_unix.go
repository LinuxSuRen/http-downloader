//go:build !windows

package cmd

import (
	"errors"
	"syscall"
)

// isAddrInUse reports whether a listen failure means the port is occupied.
func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
