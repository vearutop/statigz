package statigz_test

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vearutop/statigz"
	"github.com/vearutop/statigz/brotli"
)

//go:embed _testdata/*
var v embed.FS

func TestServer_ServeHTTP_found(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding, statigz.EncodeOnInit)

	for u, found := range map[string]bool{
		"/_testdata/favicon.png":         true,
		"/_testdata/nonexistent":         false,
		"/_testdata/swagger.json":        true,
		"/_testdata/deeper/swagger.json": true,
		"/_testdata/deeper/openapi.json": true,
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

	req, err := http.NewRequest(http.MethodGet, "/_testdata/deeper/swagger.json", nil)
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

	req, err := http.NewRequest(http.MethodGet, "/_testdata/bad.png", nil)
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "", rw.Header().Get("Etag"))
	assert.Equal(t, "failed", rw.Body.String())
}

func TestServer_ServeHTTP_head(t *testing.T) {
	s := statigz.FileServer(v, brotli.AddEncoding)

	req, err := http.NewRequest(http.MethodHead, "/_testdata/swagger.json", nil)
	require.NoError(t, err)

	req.Header.Set("Accept-Encoding", "gzip, br")

	rw := httptest.NewRecorder()
	s.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "br", rw.Header().Get("Content-Encoding"))
	assert.Equal(t, "3b88egjdndqox", rw.Header().Get("Etag"))
	assert.Len(t, rw.Body.String(), 0)
}
