package valkey

import (
	"testing"
)

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", ""},
		{"short", "a", ""},
		{"gzip magic", "\x1f\x8b\x08\x00rest", "gzip"},
		{"zstd magic", "\x28\xb5\x2f\xfdrest", "zstd"},
		{"HLL header", "HYLL\x00\x00", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectEncoding(tt.val)
			if got != tt.want {
				t.Errorf("DetectEncoding() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGzipRoundTrip(t *testing.T) {
	original := "hello, this is a test of gzip compression!"

	compressed, err := Compress(original, "gzip")
	if err != nil {
		t.Fatalf("Compress(gzip) error: %v", err)
	}

	if DetectEncoding(compressed) != "gzip" {
		t.Error("compressed data not detected as gzip")
	}

	decompressed, err := Decompress(compressed, "gzip")
	if err != nil {
		t.Fatalf("Decompress(gzip) error: %v", err)
	}

	if decompressed != original {
		t.Errorf("round-trip mismatch: got %q, want %q", decompressed, original)
	}
}

func TestZstdRoundTrip(t *testing.T) {
	original := "hello, this is a test of zstd compression!"

	compressed, err := Compress(original, "zstd")
	if err != nil {
		t.Fatalf("Compress(zstd) error: %v", err)
	}

	if DetectEncoding(compressed) != "zstd" {
		t.Error("compressed data not detected as zstd")
	}

	decompressed, err := Decompress(compressed, "zstd")
	if err != nil {
		t.Fatalf("Decompress(zstd) error: %v", err)
	}

	if decompressed != original {
		t.Errorf("round-trip mismatch: got %q, want %q", decompressed, original)
	}
}

func TestDecompressInvalidData(t *testing.T) {
	_, err := Decompress("not valid gzip data", "gzip")
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}

	_, err = Decompress("not valid zstd data", "zstd")
	if err == nil {
		t.Error("expected error for invalid zstd data")
	}
}

func TestCompressUnsupportedEncoding(t *testing.T) {
	_, err := Compress("test", "lz4")
	if err == nil {
		t.Error("expected error for unsupported encoding")
	}
}

func TestDecompressUnsupportedEncoding(t *testing.T) {
	_, err := Decompress("test", "lz4")
	if err == nil {
		t.Error("expected error for unsupported encoding")
	}
}

func TestRoundTripJSON(t *testing.T) {
	jsonData := `{"users":[{"name":"Alice","age":30},{"name":"Bob","age":25}]}`

	for _, enc := range []string{"gzip", "zstd"} {
		t.Run(enc, func(t *testing.T) {
			compressed, err := Compress(jsonData, enc)
			if err != nil {
				t.Fatalf("Compress(%s) error: %v", enc, err)
			}

			decompressed, err := Decompress(compressed, enc)
			if err != nil {
				t.Fatalf("Decompress(%s) error: %v", enc, err)
			}

			if decompressed != jsonData {
				t.Errorf("round-trip mismatch for %s", enc)
			}
		})
	}
}
