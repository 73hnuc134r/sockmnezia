//go:build !windows && !linux

package main

import "github.com/nuc134r/sockmnezia/core/engine"

func secretBox(dir string) engine.SecretBox { return plainOnly{} }
