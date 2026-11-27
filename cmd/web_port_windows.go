//go:build windows

package cmd

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isAddrInUse reports whether a listen failure means the port is occupied.
// On Windows syscall.EADDRINUSE is an invented value, while the network
// stack reports the Winsock error WSAEADDRINUSE (10048).
func isAddrInUse(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE)
}
