package statigz_test

import (
	"embed"
	"log"
	"net/http"

	"github.com/vearutop/statigz"
	"github.com/vearutop/statigz/brotli"
)

// Declare your embedded assets.

//go:embed _testdata/*
var st embed.FS

func ExampleFileServer() {
	// Plug static assets handler to your server or router.
	err := http.ListenAndServe(":80", statigz.FileServer(st, brotli.AddEncoding))
	if err != nil {
		log.Fatal(err)
	}
}
