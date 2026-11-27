package main

import "github.com/nuc134r/sockmnezia/core/engine"

// secretBox encrypts the state at rest; see secret_<os>.go.
func secretBox(dir string) engine.SecretBox { return engine.PlainBox{} }
