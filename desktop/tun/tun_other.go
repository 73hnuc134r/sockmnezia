//go:build !windows && !linux

package tun

import (
	"errors"
	"net"
)

var errUnsupported = errors.New("TUN mode is not supported on this system")

func dialHelper() (net.Conn, error)                     { return nil, errUnsupported }
func launchHelper() (<-chan error, error)               { return nil, errUnsupported }
func listenHelper([]string) (net.Listener, bool, error) { return nil, false, errUnsupported }
func NoPromptInstalled() bool                           { return false }
func SetNoPrompt(bool) error                            { return errUnsupported }
