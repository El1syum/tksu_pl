package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
)

//go:embed templates/*.html static/* static/vendor/*
var Files embed.FS

// Version invalidates browser caches whenever first-party CSS or JavaScript changes.
func Version() string {
	hash := sha256.New()
	for _, name := range []string{"static/app.css", "static/app.js", "static/stats.js", "static/chart-loader.js"} {
		data, _ := Files.ReadFile(name)
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil))[:12]
}
