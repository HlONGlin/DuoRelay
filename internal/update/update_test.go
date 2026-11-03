package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersionsUsesSemVerOrdering(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}
	for _, test := range tests {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestSelectAssetMatchesPublishedPackageNames(t *testing.T) {
	assets := []githubAsset{
		{Name: "duorelay-linux-amd64-1.2.3.tar.gz"},
		{Name: "duorelay-linux-arm64-1.2.3.tar.gz"},
		{Name: "duorelay-lite-1.2.3-amd64.tar.gz"},
		{Name: "duorelay-windows-amd64-1.2.3.zip"},
		{Name: "duorelay-lite-1.2.3-amd64.zip"},
	}
	checks := []struct {
		variant, goos, arch, want string
	}{
		{"full", "linux", "amd64", "duorelay-linux-amd64-1.2.3.tar.gz"},
		{"full", "linux", "arm64", "duorelay-linux-arm64-1.2.3.tar.gz"},
		{"lite", "linux", "amd64", "duorelay-lite-1.2.3-amd64.tar.gz"},
		{"full", "windows", "amd64", "duorelay-windows-amd64-1.2.3.zip"},
		{"lite", "windows", "amd64", "duorelay-lite-1.2.3-amd64.zip"},
	}
	for _, test := range checks {
		asset, ok := selectAsset(assets, test.variant, test.goos, test.arch, "v1.2.3")
		if !ok || asset.Name != test.want {
			t.Errorf("selectAsset(%s, %s, %s) = %#v, %v; want %s", test.variant, test.goos, test.arch, asset, ok, test.want)
		}
	}
	if _, ok := selectAsset(assets, "full", "windows", "arm64", "1.2.3"); ok {
		t.Fatal("selected an unsupported Windows architecture")
	}
	if _, ok := selectAsset(assets, "full", "linux", "amd64", "latest"); ok {
		t.Fatal("selected a non-semver release")
	}
}

func TestFetchLatestReportsMissingReleaseClearly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	service := New("1.0.0", "full")
	service.APIBaseURL = server.URL
	if _, err := service.fetchLatest(context.Background()); err == nil || !strings.Contains(err.Error(), "尚无正式发布版本") {
		t.Fatalf("fetchLatest error = %v", err)
	}
}

func TestCheckMatchesCurrentVariantAndPlatform(t *testing.T) {
	tag := "v1.2.3"
	asset, ok := selectAsset(nil, "full", runtime.GOOS, runtime.GOARCH, tag)
	if !ok {
		t.Skip("this test process runs on an unsupported update platform")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v1.2.3","name":"DuoRelay 1.2.3","body":"release notes","assets":[{"name":"`+asset.Name+`","browser_download_url":"https://example.invalid/`+asset.Name+`","size":10}]}`)
	}))
	defer server.Close()
	service := New("1.0.0", "full")
	service.APIBaseURL = server.URL
	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable || info.LatestVersion != "1.2.3" || info.AssetName != asset.Name {
		t.Fatalf("unexpected update check result: %#v", info)
	}
	if inContainer() && (info.CanUpdate || info.Reason == "") {
		t.Fatalf("container deployment should explain why in-place updates are disabled: %#v", info)
	}
	if !inContainer() && !info.CanUpdate {
		t.Fatalf("writable non-container install should allow update: %#v", info)
	}
}

func TestDownloadVerifiesSHA256AndSize(t *testing.T) {
	content := []byte("DuoRelay update package")
	hash := sha256.Sum256(content)
	digest := "sha256:" + hex.EncodeToString(hash[:])
	service := New("1.0.0", "full")
	service.Client = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(content)), ContentLength: int64(len(content)), Request: req, Header: make(http.Header)}, nil
	})}
	target := filepath.Join(t.TempDir(), "update.tar.gz")
	if err := service.download(context.Background(), "https://example.invalid/update.tar.gz", target, int64(len(content)), digest); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(t.TempDir(), "wrong.tar.gz")
	if err := service.download(context.Background(), "https://example.invalid/update.tar.gz", wrong, int64(len(content)), "sha256:deadbeef"); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("wrong digest error = %v", err)
	}
}

func TestExtractArchiveRejectsUnsafePaths(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "unsafe.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../escape", Mode: 0600, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	_ = file.Close()
	if err := extractArchive(archivePath, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("unsafe archive error = %v", err)
	}
}

func TestExtractZipRejectsSpecialFiles(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "special.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: "link", Method: zip.Store}
	header.SetMode(os.ModeSymlink | 0600)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	_ = file.Close()
	if err := extractArchive(archivePath, filepath.Join(t.TempDir(), "out")); err == nil || !strings.Contains(err.Error(), "special file") {
		t.Fatalf("special archive error = %v", err)
	}
}

func TestInstallPackageReplacesExecutableAndBundledKernels(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "package")
	packageKernels := filepath.Join(packageRoot, "kernels")
	installRoot := filepath.Join(root, "install")
	installKernels := filepath.Join(installRoot, "kernels")
	for _, dir := range []string{packageKernels, installKernels} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	name := "duorelay"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(packageRoot, name), []byte("new executable"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, name), []byte("old executable"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageKernels, "mihomo"), []byte("new kernel"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installKernels, "mihomo"), []byte("old kernel"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installKernels, "local-extra"), []byte("keep"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := installPackage(packageRoot, filepath.Join(installRoot, name)); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(installRoot, name):             "new executable",
		filepath.Join(installKernels, "mihomo"):      "new kernel",
		filepath.Join(installKernels, "local-extra"): "keep",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestValidateReleasePackageChecksVersionVariantAndPlatform(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "duorelay"), []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}
	metadata := `{"version":"1.2.3","variant":"lite","platform":"linux/amd64"}`
	if err := os.WriteFile(filepath.Join(root, "build-info.json"), []byte(metadata), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateReleasePackage(root, "linux", "amd64", "lite", "1.2.3"); err != nil {
		t.Fatalf("valid release package rejected: %v", err)
	}
	if err := validateReleasePackage(root, "linux", "amd64", "full", "1.2.3"); err == nil {
		t.Fatal("accepted package with mismatched build variant")
	}
	if err := validateReleasePackage(root, "linux", "arm64", "lite", "1.2.3"); err == nil {
		t.Fatal("accepted package with mismatched platform")
	}
	if err := validateReleasePackage(root, "linux", "amd64", "lite", "1.2.4"); err == nil {
		t.Fatal("accepted package with mismatched version")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
