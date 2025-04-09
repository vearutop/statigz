// Package zstd provides encoding for statigz.Server.
package zstd

import (
	"bytes"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/vearutop/statigz"
)

// AddEncoding is an option that prepends zstd to encodings of statigz.Server.
//
// It is located in a separate package to allow better control of imports graph.
func AddEncoding(server *statigz.Server) {
	enc := statigz.Encoding{
		FileExt:         ".zst",
		ContentEncoding: "zstd",
		Decoder: func(r io.Reader) (io.Reader, error) {
			return zstd.NewReader(r)
		},
		Encoder: func(r io.Reader) ([]byte, error) {
			res := bytes.NewBuffer(nil)

			w, err := zstd.NewWriter(res, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(w, r); err != nil {
				return nil, err
			}

			if err := w.Close(); err != nil {
				return nil, err
			}

			return res.Bytes(), nil
		},
	}

	server.Encodings = append([]statigz.Encoding{enc}, server.Encodings...)
}
