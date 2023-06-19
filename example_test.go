package statigz_test

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/vearutop/statigz"
	"github.com/vearutop/statigz/brotli"
)

// Declare your embedded assets.

//go:embed testdata/*
var st embed.FS

func ExampleFileServer() {
	s, err := fs.Sub(st, "testdata")
	if err != nil {
		log.Fatal(err)
	}

	// Plug static assets handler to your server or router.
	err = http.ListenAndServe(":80", statigz.FileServer(s.(fs.ReadDirFS), brotli.AddEncoding))
	if err != nil {
		log.Fatal(err)
	}
}
