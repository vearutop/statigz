package brotli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vearutop/statigz"
	"github.com/vearutop/statigz/brotli"
)

func TestAddEncoding(t *testing.T) {
	s := &statigz.Server{}
	s.Encodings = append(s.Encodings, statigz.GzipEncoding())
	brotli.AddEncoding(s)

	assert.Equal(t, ".br", s.Encodings[0].FileExt)
	assert.Equal(t, ".gz", s.Encodings[1].FileExt)
	d, err := s.Encodings[0].Decoder(nil)
	assert.NoError(t, err)
	assert.NotNil(t, d)
}
