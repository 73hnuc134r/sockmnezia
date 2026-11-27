//go:build !windows && !linux

package main

func platformProxy() osProxy { return nil }
