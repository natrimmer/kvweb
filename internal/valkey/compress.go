package valkey

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// DetectEncoding checks magic bytes and returns "gzip", "zstd", or "".
func DetectEncoding(val string) string {
	if len(val) >= 2 && val[0] == 0x1f && val[1] == 0x8b {
		return "gzip"
	}
	if len(val) >= 4 && val[0] == 0x28 && val[1] == 0xb5 && val[2] == 0x2f && val[3] == 0xfd {
		return "zstd"
	}
	return ""
}

// Decompress decompresses val using the specified encoding ("gzip" or "zstd").
func Decompress(val string, encoding string) (string, error) {
	switch encoding {
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader([]byte(val)))
		if err != nil {
			return "", err
		}
		defer func() { _ = r.Close() }()
		out, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		return string(out), nil
	case "zstd":
		d, err := zstd.NewReader(bytes.NewReader([]byte(val)))
		if err != nil {
			return "", err
		}
		defer d.Close()
		out, err := io.ReadAll(d)
		if err != nil {
			return "", err
		}
		return string(out), nil
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// Compress compresses val using the specified encoding ("gzip" or "zstd").
func Compress(val string, encoding string) (string, error) {
	switch encoding {
	case "gzip":
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write([]byte(val)); err != nil {
			return "", err
		}
		if err := w.Close(); err != nil {
			return "", err
		}
		return buf.String(), nil
	case "zstd":
		var buf bytes.Buffer
		w, err := zstd.NewWriter(&buf)
		if err != nil {
			return "", err
		}
		if _, err := w.Write([]byte(val)); err != nil {
			_ = w.Close()
			return "", err
		}
		if err := w.Close(); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}
}
