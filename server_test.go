package statigz_test

import (
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	brotli2 "github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vearutop/statigz"
	"github.com/vearutop/statigz/brotli"
)

//go:embed testdata/*
var v embed.FS

func TestServer_ServeHTTP_std(t *testing.T) {
	s := http.FileServer(http.FS(v))

	for u, found := range map[string]bool{
		"/testdata/favicon.png":         true,
		"/testdata/nonexistent":         false,
		"/testdata/swagger.json":        true,
		"/testdata/deeper/swagger.json": false,
		"/testdata/deeper/openapi.json": false,
		"/testdata/":                    true,
		"/testdata/?abc":                true,
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		if found {
			assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
			assert.Equal(t, http.StatusOK, rw.Code, u)
		} else {
			assert.Equal(t, http.StatusNotFound, rw.Code, u)
		}
	}

	for u, l := range map[string]string{
		"/testdata/index.html": "./",
		"/testdata":            "testdata/",
		"/testdata?abc":        "testdata/?abc",
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		assert.Equal(t, http.StatusMovedPermanently, rw.Code, u)
		assert.Equal(t, l, rw.Header().Get("Location"))
	}
}

func TestServer_ServeHTTP_found(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding, statigz.EncodeOnInit)

	for u, found := range map[string]bool{
		"/testdata/favicon.png":         true,
		"/testdata/nonexistent":         false,
		"/testdata/swagger.json":        true,
		"/testdata/deeper/swagger.json": true,
		"/testdata/deeper/openapi.json": true,
		"/testdata/":                    true,
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		if found {
			assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
			assert.Equal(t, http.StatusOK, rw.Code, u)
		} else {
			assert.Equal(t, http.StatusNotFound, rw.Code, u)
		}
	}

	for u, l := range map[string]string{
		"/testdata/index.html": "./",
		"/testdata":            "testdata/",
		"/testdata?abc":        "testdata/?abc",
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		assert.Equal(t, http.StatusMovedPermanently, rw.Code, u)
		assert.Equal(t, l, rw.Header().Get("Location"))
	}
}

func TestServer_ServeHTTP_error(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding)

	req, err := http.NewRequest(http.MethodDelete, "/", nil)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rw.Code)
	assert.Equal(t, "Method Not Allowed\n\nmethod should be GET or HEAD\n", rw.Body.String())
}

func TestServer_ServeHTTP_acceptEncoding(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding, statigz.EncodeOnInit)

	req, err := http.NewRequest(http.MethodGet, "/testdata/deeper/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "br", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "3b88egjdndqox", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.Bytes(), 2548)

	req.Header.Set("Accept-Encoding", "gzip")

	rw = httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "3b88egjdndqoxU", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.Bytes(), 24919)

	req.Header.Set("Accept-Encoding", "gzip, br")
	req.Header.Set("If-None-Match", "3b88egjdndqox")

	rw = httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusNotModified, rw.Code)
	assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.Bytes(), 0)
}

func TestServer_ServeHTTP_badFile(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding,
		statigz.OnError(func(rw http.ResponseWriter, r *http.Request, err error) {
			assert.EqualError(t, err, "gzip: invalid header")

			_, err = rw.Write([]byte("failed"))
			assert.NoError(t, err)
		}))

	req, err := http.NewRequest(http.MethodGet, "/testdata/bad.png", nil)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "", rw.Header().Get("Etag"))
	assert.Equal(t, "failed", rw.Body.String())
}

func TestServer_ServeHTTP_head(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding, statigz.EncodeOnInit)

	req, err := http.NewRequest(http.MethodHead, "/testdata/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "br", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "1bp69hxb9nd93.br", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.String(), 0)
}

func TestServer_ServeHTTP_head_gz(t *testing.T) {
	s := statigz.FileServer(v, statigz.EncodeOnInit)

	req, err := http.NewRequest(http.MethodHead, "/testdata/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "gzip", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "1bp69hxb9nd93.gz", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.String(), 0)
}

func BenchmarkServer_ServeHTTP(b *testing.B) {
	s := statigz.FileServer(v, statigz.EncodeOnInit)

	req, err := http.NewRequest(http.MethodGet, "/testdata/swagger.json", nil)
	require.NoError(b, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)
	}
}

func TestServer_ServeHTTP_get_gz(t *testing.T) {
	s := statigz.FileServer(v, statigz.EncodeOnInit)

	req, err := http.NewRequest(http.MethodGet, "/testdata/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "gzip", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "1bp69hxb9nd93.gz", rw.Header().Get("Etag"))
	assert.Equal(t, "Accept-Encoding", rw.Header().Get("Vary"))
	assert.NotEmpty(t, rw.Body.String())

	r, err := gzip.NewReader(rw.Body)
	assert.NoError(t, err)

	decoded, err := io.ReadAll(r)
	assert.NoError(t, err)

	raw, err := os.ReadFile("testdata/swagger.json")
	assert.NoError(t, err)

	assert.Equal(t, raw, decoded)
}

func TestServer_ServeHTTP_get_br(t *testing.T) {
	s := statigz.FileServer(v, statigz.EncodeOnInit, brotli.AddEncoding)

	req, err := http.NewRequest(http.MethodGet, "/testdata/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "br", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "1bp69hxb9nd93.br", rw.Header().Get("Etag"))
	assert.NotEmpty(t, rw.Body.String())

	r := brotli2.NewReader(rw.Body)

	decoded, err := io.ReadAll(r)
	assert.NoError(t, err)

	raw, err := os.ReadFile("testdata/swagger.json")
	assert.NoError(t, err)

	assert.Equal(t, raw, decoded)
}

func TestServer_ServeHTTP_indexCompressed(t *testing.T) {
	s := statigz.FileServer(v)

	req, err := http.NewRequest(http.MethodGet, "/testdata/", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "gzip", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "45pls0g4wm91", rw.Header().Get("Etag"))
	assert.NotEmpty(t, rw.Body.String())

	r, err := gzip.NewReader(rw.Body)
	assert.NoError(t, err)

	decoded, err := io.ReadAll(r)
	assert.NoError(t, err)

	assert.Equal(t, "Hello!", string(decoded))
}

func TestServer_ServeHTTP_sub(t *testing.T) {
	vs, err := fs.Sub(v, "testdata")
	require.NoError(t, err)

	s := statigz.FileServer(vs.(fs.ReadDirFS), brotli.AddEncoding, statigz.EncodeOnInit)

	for u, found := range map[string]bool{
		"/favicon.png":         true,
		"/nonexistent":         false,
		"/swagger.json":        true,
		"/deeper/swagger.json": true,
		"/deeper/openapi.json": true,
		"/":                    true,
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		if found {
			assert.True(t, s.Found(req))
			assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
			assert.Equal(t, http.StatusOK, rw.Code, u)
		} else {
			assert.False(t, s.Found(req))
			assert.Equal(t, http.StatusNotFound, rw.Code, u)
		}
	}

	for u, l := range map[string]string{
		"/index.html": "./",
		"/deeper":     "deeper/",
		"/deeper?abc": "deeper/?abc",
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		require.NoError(t, err)

		rw := httptest.NewRecorder()
		s.ServeHTTP(rw, req)

		assert.Equal(t, http.StatusMovedPermanently, rw.Code, u)
		assert.Equal(t, l, rw.Header().Get("Location"))
	}
}

func ExampleServer_Found() {
	fileServer := statigz.FileServer(st)
	customHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve existing static resource.
		if fileServer.Found(r) {
			fileServer.ServeHTTP(w, r)

			return
		}

		// Do something custom for non-existing resource, for example serve index page.
		// (This is an example, serving index instead of 404 might not be the best idea in real life 😅).
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	// Plug static assets handler to your server or router.
	if err := http.ListenAndServe("localhost:80", customHandler); err != nil {
		log.Fatal(err)
	}
}
