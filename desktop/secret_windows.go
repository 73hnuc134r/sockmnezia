package main

import (
	"bytes"
	"unsafe"

	"github.com/nuc134r/sockmnezia/core/engine"
	"golang.org/x/sys/windows"
)

// dpapiBox encrypts with DPAPI in the user's scope: only this Windows account can decrypt.
type dpapiBox struct{}

func secretBox(dir string) engine.SecretBox {
	secretStore = "dpapi"
	return dpapiBox{}
}

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func takeBlob(out *windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return bytes.Clone(unsafe.Slice(out.Data, out.Size))
}

var dpapiEntropy = []byte("sockmnezia desktop state")

func (dpapiBox) Seal(p []byte) ([]byte, error) {
	var out windows.DataBlob
	if err := windows.CryptProtectData(blob(p), nil, blob(dpapiEntropy), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	return append(append([]byte{}, magicDPAPI...), takeBlob(&out)...), nil
}

func (dpapiBox) Open(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, magicDPAPI) {
		return plainOnly{}.Open(b)
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(blob(b[len(magicDPAPI):]), nil, blob(dpapiEntropy), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	return takeBlob(&out), nil
}
