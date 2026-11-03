package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DefaultRepository = "HlONGlin/DuoRelay"
	maxDownloadSize   = int64(1024 * 1024 * 1024)
	maxArchiveSize    = int64(2 * 1024 * 1024 * 1024)
)

var ErrInProgress = errors.New("an update is already in progress")

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
var repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type ReleaseInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	ReleaseName     string `json:"release_name,omitempty"`
	ReleaseURL      string `json:"release_url,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	CanUpdate       bool   `json:"can_update"`
	Reason          string `json:"reason,omitempty"`
}

type ApplyResult struct {
	LatestVersion string `json:"latest_version"`
	AssetName     string `json:"asset_name"`
	Restarting    bool   `json:"restarting"`
	Message       string `json:"message"`
}

type ApplyProgress struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type Service struct {
	CurrentVersion string
	Variant        string
	Repository     string
	APIBaseURL     string
	Client         *http.Client
	Restart        func()
	applyMu        sync.Mutex
	pending        *updateManifest
	progressMu     sync.RWMutex
	progress       ApplyProgress
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type updateManifest struct {
	ParentPID  int      `json:"parent_pid"`
	Stage      string   `json:"stage"`
	CleanupDir string   `json:"cleanup_dir"`
	Target     string   `json:"target"`
	Args       []string `json:"args"`
	Env        []string `json:"env"`
	Dir        string   `json:"dir"`
	Restart    bool     `json:"restart"`
}

func New(currentVersion, variantName string) *Service {
	repository := strings.TrimSpace(os.Getenv("DUORELAY_UPDATE_REPO"))
	if repository == "" {
		repository = DefaultRepository
	}
	return &Service{
		CurrentVersion: currentVersion,
		Variant:        variantName,
		Repository:     repository,
		Client:         &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) StartApply() (*ApplyProgress, error) {
	s.progressMu.Lock()
	if s.progress.State == "downloading" || s.progress.State == "restarting" {
		s.progressMu.Unlock()
		return nil, ErrInProgress
	}
	s.progress = ApplyProgress{State: "downloading", Message: "正在下载并验证更新包"}
	s.progressMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, err := s.Apply(ctx)
		s.progressMu.Lock()
		defer s.progressMu.Unlock()
		if err != nil {
			s.progress = ApplyProgress{State: "failed", Message: err.Error()}
			return
		}
		s.progress = ApplyProgress{State: "restarting", Message: result.Message}
	}()
	status := s.Status()
	return &status, nil
}

func (s *Service) Status() ApplyProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	if s.progress.State == "" {
		return ApplyProgress{State: "idle"}
	}
	return s.progress
}

func (s *Service) Check(ctx context.Context) (*ReleaseInfo, error) {
	release, err := s.fetchLatest(ctx)
	if err != nil {
		return nil, err
	}
	asset, hasAsset := selectAsset(release.Assets, s.Variant, runtime.GOOS, runtime.GOARCH, release.TagName)
	current := strings.TrimSpace(s.CurrentVersion)
	latest := normalizeVersion(release.TagName)
	if !validVersion(latest) {
		return nil, errors.New("update service returned an invalid version")
	}
	info := &ReleaseInfo{
		CurrentVersion:  current,
		LatestVersion:   latest,
		ReleaseName:     release.Name,
		ReleaseURL:      release.HTMLURL,
		PublishedAt:     release.PublishedAt,
		ReleaseNotes:    truncate(release.Body, 8000),
		UpdateAvailable: compareVersions(latest, current) > 0,
		AssetName:       asset.Name,
		CanUpdate:       false,
	}
	if !info.UpdateAvailable {
		info.Reason = "当前已是最新版本"
		return info, nil
	}
	if !hasAsset {
		info.Reason = fmt.Sprintf("当前平台和 %s 变体没有匹配的发布包", s.Variant)
		return info, nil
	}
	info.CanUpdate, info.Reason = writableExecutable()
	return info, nil
}

func (s *Service) Apply(ctx context.Context) (*ApplyResult, error) {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	info, err := s.Check(ctx)
	if err != nil {
		return nil, err
	}
	if !info.UpdateAvailable {
		return nil, errors.New("当前已是最新版本")
	}
	if !info.CanUpdate {
		return nil, errors.New(info.Reason)
	}
	release, err := s.fetchLatest(ctx)
	if err != nil {
		return nil, err
	}
	asset, ok := selectAsset(release.Assets, s.Variant, runtime.GOOS, runtime.GOARCH, release.TagName)
	if !ok || asset.URL == "" {
		return nil, errors.New("latest release has no matching package")
	}
	if asset.Size > maxDownloadSize {
		return nil, errors.New("update package exceeds the size limit")
	}
	stage, err := os.MkdirTemp("", "duorelay-update-")
	if err != nil {
		return nil, err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	archivePath := filepath.Join(stage, filepath.Base(asset.Name))
	if err := s.download(ctx, asset.URL, archivePath, asset.Size, asset.Digest); err != nil {
		return nil, err
	}
	extracted := filepath.Join(stage, "package")
	if err := extractArchive(archivePath, extracted); err != nil {
		return nil, err
	}
	packageRoot, err := findPackageRoot(extracted)
	if err != nil {
		return nil, err
	}
	if err := validateReleasePackage(packageRoot, runtime.GOOS, runtime.GOARCH, s.Variant, info.LatestVersion); err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	manifest := updateManifest{
		ParentPID:  os.Getpid(),
		Stage:      packageRoot,
		CleanupDir: stage,
		Target:     executable,
		Args:       append([]string(nil), os.Args[1:]...),
		Env:        os.Environ(),
		Dir:        currentDir(),
		Restart:    runtime.GOOS == "windows",
	}
	if err := s.startUpdate(manifest, stage); err != nil {
		return nil, err
	}
	keepStage = true
	result := &ApplyResult{LatestVersion: info.LatestVersion, AssetName: asset.Name, Restarting: true, Message: "更新包已下载，服务即将重启"}
	return result, nil
}

func (s *Service) startUpdate(manifest updateManifest, stage string) error {
	if runtime.GOOS != "windows" {
		if s.Restart == nil {
			return errors.New("update restart callback is not configured")
		}
		s.pending = &manifest
		if s.Restart != nil {
			go func() {
				time.Sleep(1500 * time.Millisecond)
				s.Restart()
			}()
		}
		return nil
	}
	if s.Restart == nil {
		return errors.New("update restart callback is not configured")
	}
	helper := filepath.Join(stage, helperName())
	if err := copyFile(manifest.Target, helper, 0755); err != nil {
		return fmt.Errorf("prepare update helper: %w", err)
	}
	manifestPath := filepath.Join(stage, "manifest.json")
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		return err
	}
	cmd := exec.Command(helper, "--apply-update", manifestPath)
	cmd.Dir = stage
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start update helper: %w", err)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		s.Restart()
	}()
	return nil
}

// Finalize installs and execs the staged update from the process shutdown path.
// On Unix, renaming over the running executable is atomic, and exec keeps the
// same PID so service managers continue tracking the updated process.
func (s *Service) Finalize() error {
	if s.pending == nil {
		return nil
	}
	manifest := *s.pending
	if err := installPackage(manifest.Stage, manifest.Target); err != nil {
		return fmt.Errorf("install update package: %w", err)
	}
	_ = os.RemoveAll(manifest.CleanupDir)
	args := append([]string{manifest.Target}, manifest.Args...)
	return syscall.Exec(manifest.Target, args, manifest.Env)
}

func (s *Service) fetchLatest(ctx context.Context) (*githubRelease, error) {
	repository := strings.Trim(strings.TrimSpace(s.Repository), "/")
	repository = strings.TrimPrefix(repository, "https://github.com/")
	if !repositoryPattern.MatchString(repository) {
		return nil, errors.New("update repository must be a GitHub owner/repository")
	}
	base := strings.TrimRight(s.APIBaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+repository+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DuoRelay-update-check")
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New("当前更新仓库尚无正式发布版本，或仓库地址不可用")
		}
		return nil, fmt.Errorf("update service returned %s", resp.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); err != nil {
		return nil, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return nil, errors.New("update service returned an invalid release")
	}
	return &release, nil
}

func (s *Service) download(ctx context.Context, url, target string, expectedSize int64, digest string) error {
	if !strings.HasPrefix(url, "https://") {
		return errors.New("update package URL must use HTTPS")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "DuoRelay-update")
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	priorRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if !strings.EqualFold(next.URL.Scheme, "https") {
			return errors.New("update package redirects must use HTTPS")
		}
		if priorRedirect != nil {
			return priorRedirect(next, via)
		}
		if len(via) >= 10 {
			return errors.New("too many update package redirects")
		}
		return nil
	}
	resp, err := clientCopy.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update package: %s", resp.Status)
	}
	if resp.Request != nil && resp.Request.URL.Scheme != "https" {
		return errors.New("update package redirects must use HTTPS")
	}
	if resp.ContentLength > maxDownloadSize {
		return errors.New("update package exceeds the size limit")
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxDownloadSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxDownloadSize || (expectedSize > 0 && written != expectedSize) {
		return errors.New("downloaded update package has an invalid size")
	}
	if digest != "" {
		want := strings.TrimPrefix(strings.TrimSpace(digest), "sha256:")
		if !strings.EqualFold(want, hex.EncodeToString(hash.Sum(nil))) {
			return errors.New("downloaded update package failed SHA-256 verification")
		}
	}
	return nil
}

func selectAsset(assets []githubAsset, variantName, goos, goarch, tag string) (githubAsset, bool) {
	if !validVersion(normalizeVersion(tag)) {
		return githubAsset{}, false
	}
	version, arch := normalizeVersion(tag), goarch
	if goos == "windows" {
		if goarch != "amd64" {
			return githubAsset{}, false
		}
		arch = "amd64"
	} else if goos != "linux" || (goarch != "amd64" && goarch != "arm64") {
		return githubAsset{}, false
	}
	var want string
	if variantName == "lite" {
		if goos == "windows" {
			want = fmt.Sprintf("duorelay-lite-%s-%s.zip", version, arch)
		} else {
			want = fmt.Sprintf("duorelay-lite-%s-%s.tar.gz", version, arch)
		}
	} else if goos == "windows" {
		want = fmt.Sprintf("duorelay-windows-%s-%s.zip", arch, version)
	} else {
		want = fmt.Sprintf("duorelay-linux-%s-%s.tar.gz", arch, version)
	}
	for _, asset := range assets {
		if asset.Name == want {
			return asset, true
		}
	}
	return githubAsset{}, false
}

func normalizeVersion(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "refs/tags/")
	return strings.TrimPrefix(value, "v")
}

func compareVersions(left, right string) int {
	left, right = normalizeVersion(left), normalizeVersion(right)
	if right == "" || right == "dev" || right == "unknown" {
		if left == "" {
			return 0
		}
		return 1
	}
	if left == "" || left == "dev" || left == "unknown" {
		return -1
	}
	lv, lp, _ := parseVersion(left)
	rv, rp, _ := parseVersion(right)
	for i := range lv {
		if lv[i] < rv[i] {
			return -1
		}
		if lv[i] > rv[i] {
			return 1
		}
	}
	if len(lp) == 0 && len(rp) > 0 {
		return 1
	}
	if len(lp) > 0 && len(rp) == 0 {
		return -1
	}
	for i := 0; i < len(lp) && i < len(rp); i++ {
		ln, le := strconv.ParseUint(lp[i], 10, 64)
		rn, re := strconv.ParseUint(rp[i], 10, 64)
		switch {
		case le == nil && re == nil:
			if ln < rn {
				return -1
			}
			if ln > rn {
				return 1
			}
		case le == nil:
			return -1
		case re == nil:
			return 1
		default:
			if cmp := strings.Compare(lp[i], rp[i]); cmp != 0 {
				return cmp
			}
		}
	}
	if len(lp) < len(rp) {
		return -1
	}
	if len(lp) > len(rp) {
		return 1
	}
	return 0
}

func parseVersion(value string) ([3]uint64, []string, bool) {
	var numbers [3]uint64
	match := semverPattern.FindStringSubmatch(value)
	if match == nil {
		return numbers, nil, false
	}
	for i, component := range match[1:4] {
		number, err := strconv.ParseUint(component, 10, 64)
		if err != nil {
			return numbers, nil, false
		}
		numbers[i] = number
	}
	var prerelease []string
	if match[4] != "" {
		prerelease = strings.Split(match[4], ".")
		for _, identifier := range prerelease {
			if identifier == "" || (allDigits(identifier) && len(identifier) > 1 && identifier[0] == '0') {
				return numbers, nil, false
			}
		}
	}
	return numbers, prerelease, true
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return value != ""
}

func validVersion(value string) bool {
	if value == "" || value == "dev" || value == "unknown" {
		return false
	}
	_, _, ok := parseVersion(value)
	return ok
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func writableExecutable() (bool, string) {
	if inContainer() {
		return false, "容器部署请通过更新镜像升级，不能在容器内替换程序"
	}
	executable, err := os.Executable()
	if err != nil {
		return false, "无法定位当前程序"
	}
	if executable, err = filepath.EvalSymlinks(executable); err != nil {
		return false, "无法解析当前程序安装路径"
	}
	test, err := os.CreateTemp(filepath.Dir(executable), ".duorelay-write-check-*")
	if err != nil {
		return false, "当前程序目录不可写，请使用有权限的更新方式"
	}
	name := test.Name()
	_ = test.Close()
	_ = os.Remove(name)
	return true, ""
}

func inContainer() bool {
	if strings.TrimSpace(os.Getenv("container")) != "" || strings.TrimSpace(os.Getenv("KUBERNETES_SERVICE_HOST")) != "" {
		return true
	}
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

func extractArchive(archivePath, destination string) error {
	if err := os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destination)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := gzip.NewReader(io.LimitReader(file, maxArchiveSize))
	if err != nil {
		return err
	}
	defer reader.Close()
	tr := tar.NewReader(io.LimitReader(reader, maxArchiveSize))
	var totalSize int64
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name, err := safeArchivePath(header.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > maxArchiveSize || totalSize > maxArchiveSize-header.Size {
				return errors.New("update archive exceeds the extraction limit")
			}
			totalSize += header.Size
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0755)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(out, tr, header.Size)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return errors.New("update archive contains an unsupported file type")
		}
	}
	return nil
}

func extractZip(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	var totalSize uint64
	for _, entry := range archive.File {
		name, err := safeArchivePath(entry.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			continue
		}
		if !entry.Mode().IsRegular() || entry.UncompressedSize64 > uint64(maxArchiveSize) || totalSize > uint64(maxArchiveSize)-entry.UncompressedSize64 {
			return errors.New("update archive exceeds the extraction limit or contains a special file")
		}
		totalSize += entry.UncompressedSize64
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(in, maxArchiveSize))
		_ = in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func safeArchivePath(name string) (string, error) {
	if strings.ContainsRune(name, '\\') || strings.ContainsRune(name, 0) {
		return "", errors.New("update archive contains an unsafe path")
	}
	name = filepath.Clean(filepath.FromSlash(name))
	if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
		return "", errors.New("update archive contains an unsafe path")
	}
	return name, nil
}

func findPackageRoot(base string) (string, error) {
	name := "duorelay"
	if runtime.GOOS == "windows" {
		name = "duorelay.exe"
	}
	var found string
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name && found == "" {
			found = filepath.Dir(path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("update archive does not contain the DuoRelay executable")
	}
	return found, nil
}

func validatePackage(root, goos string) error {
	name := "duorelay"
	if goos == "windows" {
		name = "duorelay.exe"
	}
	info, err := os.Stat(filepath.Join(root, name))
	if err != nil || info.Size() == 0 {
		return errors.New("update package contains an invalid executable")
	}
	return nil
}

func validateReleasePackage(root, goos, arch, variantName, version string) error {
	if err := validatePackage(root, goos); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "build-info.json"))
	if err != nil {
		return fmt.Errorf("update package is missing build metadata: %w", err)
	}
	var buildInfo struct {
		Version  string `json:"version"`
		Variant  string `json:"variant"`
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal(data, &buildInfo); err != nil {
		return fmt.Errorf("update package has invalid build metadata: %w", err)
	}
	if normalizeVersion(buildInfo.Version) != normalizeVersion(version) || buildInfo.Variant != variantName || buildInfo.Platform != goos+"/"+arch {
		return errors.New("update package does not match the release version, build variant and platform")
	}
	return nil
}

func copyFile(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func stagedCopy(source, directory string, mode os.FileMode) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(directory, ".duorelay-update-stage-*")
	if err != nil {
		return "", err
	}
	temp := out.Name()
	cleanup := func() { _ = out.Close(); _ = os.Remove(temp) }
	if err := out.Chmod(mode); err != nil {
		cleanup()
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		cleanup()
		return "", err
	}
	if err := out.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(temp)
		return "", err
	}
	return temp, nil
}

func helperName() string {
	if runtime.GOOS == "windows" {
		return "duorelay-update-helper.exe"
	}
	return "duorelay-update-helper"
}
func currentDir() string { value, _ := os.Getwd(); return value }

func RunHelper(manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest updateManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if err := waitForProcess(manifest.ParentPID, 30*time.Second); err != nil {
		return err
	}
	if err := installPackage(manifest.Stage, manifest.Target); err != nil {
		return err
	}
	if !manifest.Restart {
		_ = os.RemoveAll(manifest.CleanupDir)
		return nil
	}
	cmd := exec.Command(manifest.Target, manifest.Args...)
	cmd.Env, cmd.Dir = manifest.Env, manifest.Dir
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = os.RemoveAll(manifest.CleanupDir)
	return nil
}

func waitForProcess(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("timed out waiting for the previous DuoRelay process")
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
		return err == nil && strings.Contains(string(output), strconv.Itoa(pid))
	}
	process, err := os.FindProcess(pid)
	return err == nil && process.Signal(syscall.Signal(0)) == nil
}

func installPackage(source, target string) error {
	name := "duorelay"
	if runtime.GOOS == "windows" {
		name = "duorelay.exe"
	}
	newExecutable := filepath.Join(source, name)
	if _, err := os.Stat(newExecutable); err != nil {
		return err
	}
	executableTemp, err := stagedCopy(newExecutable, filepath.Dir(target), 0755)
	if err != nil {
		return fmt.Errorf("stage updated executable: %w", err)
	}
	defer os.Remove(executableTemp)
	type replacement struct {
		target        string
		staged        string
		backup        string
		hadOriginal   bool
		movedOriginal bool
		installed     bool
	}
	var replacements []replacement
	rollback := func() {
		for index := len(replacements) - 1; index >= 0; index-- {
			item := replacements[index]
			if item.movedOriginal {
				_ = os.Remove(item.target)
				_ = os.Rename(item.backup, item.target)
			} else if item.installed {
				_ = os.Remove(item.target)
			}
			_ = os.Remove(item.staged)
			_ = os.Remove(item.backup)
		}
	}
	newKernels := filepath.Join(source, "kernels")
	entries, readErr := os.ReadDir(newKernels)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if readErr == nil {
		targetKernels := filepath.Join(filepath.Dir(target), "kernels")
		if err := os.MkdirAll(targetKernels, 0755); err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !entry.Type().IsRegular() && entry.Type() != 0 {
				rollback()
				return errors.New("update package contains a non-regular kernel file")
			}
			kernelTarget := filepath.Join(targetKernels, entry.Name())
			staged, err := stagedCopy(filepath.Join(newKernels, entry.Name()), targetKernels, 0755)
			if err != nil {
				rollback()
				return fmt.Errorf("stage updated kernel %s: %w", entry.Name(), err)
			}
			item := replacement{target: kernelTarget, staged: staged}
			if _, err := os.Lstat(kernelTarget); err == nil {
				info, statErr := os.Stat(kernelTarget)
				if statErr != nil || !info.Mode().IsRegular() {
					_ = os.Remove(staged)
					rollback()
					return fmt.Errorf("existing kernel target %s is not a regular file", entry.Name())
				}
				backup, backupErr := os.CreateTemp(targetKernels, ".duorelay-update-backup-*")
				if backupErr != nil {
					_ = os.Remove(staged)
					rollback()
					return fmt.Errorf("prepare backup for kernel %s: %w", entry.Name(), backupErr)
				}
				item.backup = backup.Name()
				_ = backup.Close()
				_ = os.Remove(item.backup)
				item.hadOriginal = true
			} else if !errors.Is(err, os.ErrNotExist) {
				_ = os.Remove(staged)
				rollback()
				return err
			}
			replacements = append(replacements, item)
		}
	}
	if _, err := os.Lstat(target); err == nil {
		backup, backupErr := os.CreateTemp(filepath.Dir(target), ".duorelay-update-backup-*")
		if backupErr != nil {
			rollback()
			return fmt.Errorf("prepare executable backup: %w", backupErr)
		}
		item := replacement{target: target, staged: executableTemp, backup: backup.Name(), hadOriginal: true}
		_ = backup.Close()
		_ = os.Remove(item.backup)
		replacements = append(replacements, item)
	} else if !errors.Is(err, os.ErrNotExist) {
		rollback()
		return err
	} else {
		replacements = append(replacements, replacement{target: target, staged: executableTemp})
	}
	for index := range replacements {
		item := &replacements[index]
		if item.hadOriginal {
			if err := os.Rename(item.target, item.backup); err != nil {
				rollback()
				return fmt.Errorf("backup existing file %s: %w", filepath.Base(item.target), err)
			}
			item.movedOriginal = true
		}
		if err := os.Rename(item.staged, item.target); err != nil {
			rollback()
			return fmt.Errorf("replace kernel %s: %w", filepath.Base(item.target), err)
		}
		item.installed = true
	}
	for _, item := range replacements {
		_ = os.Remove(item.backup)
	}
	return nil
}
