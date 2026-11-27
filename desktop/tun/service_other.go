//go:build !windows && !linux

package tun

import (
	"errors"
	"net"
)

var errUnsupported = errors.New("TUN mode is not supported on this system")

func dialService() (net.Conn, error)    { return nil, errUnsupported }
func Installed() bool                   { return false }
func runPrivileged(string) error        { return errUnsupported }
func RunInstall([]string) error         { return errUnsupported }
func RunUninstall([]string) error       { return errUnsupported }
func RunService(string, []string) error { return errUnsupported }
