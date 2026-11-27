package main

import (
	"encoding/base64"
	"errors"
	"runtime"

	"rsc.io/qr"
)

// Desktop holds what only the desktop app has (version, platform features).
type Desktop struct {
	version string
}

// Info describes the app build.
type Info struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Xray    string `json:"xray"`
}

func (d *Desktop) Info() Info { return Info{Version: d.version, OS: runtime.GOOS, Xray: xrayVersion()} }

// errTooLong: the text does not fit in a QR code.
var errTooLong = errors.New("too long for a QR code")

// QR returns text as a QR code PNG (data: URL). Generated locally: shared links hold credentials.
func (d *Desktop) QR(text string) (string, error) {
	if len(text) > 2000 {
		return "", errTooLong
	}
	c, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	c.Scale = 6
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(c.PNG()), nil
}
