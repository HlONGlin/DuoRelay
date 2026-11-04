package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadVariants(t *testing.T) {
	for _, tc := range []struct{ variant, arch string }{{"full", "amd64"}, {"full", "arm64"}, {"lite", "amd64"}, {"lite", "arm64"}} {
		t.Run(tc.variant+"/"+tc.arch, func(t *testing.T) {
			t.Chdir(t.TempDir())
			data := []byte("temporary kernel fixture")
			hash := fmt.Sprintf("%x", sha256.Sum256(data))
			lock := fmt.Sprintf("mihomo:\n  version: '1'\n  sha256:\n    linux-%s: '%s'\n", tc.arch, hash)
			if tc.variant == "full" {
				lock += fmt.Sprintf("singbox:\n  version: '1'\n  sha256:\n    linux-%s: '%s'\ncronet:\n  linux-%s:\n    url: 'https://fixture.invalid/libcronet.so'\n    sha256: '%s'\n", tc.arch, hash, tc.arch, hash)
			}
			if err := os.WriteFile("kernels.lock", []byte(lock), 0600); err != nil {
				t.Fatal(err)
			}
			var requests []string
			oldTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = oldTransport })
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests = append(requests, r.URL.String())
				var body bytes.Buffer
				if strings.Contains(r.URL.Path, "sing-box") {
					gz := gzip.NewWriter(&body)
					tr := tar.NewWriter(gz)
					if err := tr.WriteHeader(&tar.Header{Name: "fixture/sing-box", Mode: 0755, Size: int64(len(data))}); err != nil {
						t.Fatal(err)
					}
					_, _ = tr.Write(data)
					_ = tr.Close()
					_ = gz.Close()
				} else if strings.HasSuffix(r.URL.Path, ".gz") {
					gz := gzip.NewWriter(&body)
					_, _ = gz.Write(data)
					_ = gz.Close()
				} else {
					body.Write(data)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(&body)}, nil
			})
			if err := download(tc.arch, "out", tc.variant); err != nil {
				t.Fatal(err)
			}
			want := 1
			if tc.variant == "full" {
				want = 3
			}
			files, err := os.ReadDir("out")
			if err != nil || len(files) != want || len(requests) != want {
				t.Fatalf("files=%v requests=%v error=%v", files, requests, err)
			}
			for _, file := range files {
				got, err := os.ReadFile(filepath.Join("out", file.Name()))
				if err != nil || !bytes.Equal(got, data) || (tc.variant == "lite" && file.Name() != "mihomo") {
					t.Fatalf("unexpected file %s: %v", file.Name(), err)
				}
			}
			if err := os.WriteFile("kernels.lock", []byte(strings.ReplaceAll(lock, hash, strings.Repeat("0", 64))), 0600); err != nil {
				t.Fatal(err)
			}
			if err := download(tc.arch, "bad-checksum", tc.variant); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
				t.Fatalf("accepted corrupt binary: %v", err)
			}
		})
	}
}

func TestDownloadRejectsInvalidTargetOrMissingPin(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("kernels.lock", []byte("mihomo:\n  version: '1'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ arch, variant string }{{"amd64", "invalid"}, {"386", "lite"}, {"amd64", "lite"}, {"arm64", "full"}} {
		if err := download(tc.arch, "out", tc.variant); err == nil {
			t.Fatalf("accepted unsupported or unpinned target: %+v", tc)
		}
	}
}
