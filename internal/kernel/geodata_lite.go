//go:build lite

package kernel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// These are the sources used by mihomo itself (geodata-mode defaults to false).
// The -d directory is also mihomo's cache directory, including for real validation.
var geoAssets = map[string]string{
	"geoip.metadb": "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb",
	"GeoSite.dat":  "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
}

func PrepareData(ctx context.Context, directory string) error {
	client := &http.Client{Timeout: 3 * time.Minute}
	return prepareGeoData(ctx, client, directory, geoAssets)
}

func prepareGeoData(ctx context.Context, client *http.Client, directory string, assets map[string]string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	for name, url := range assets {
		path := filepath.Join(directory, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			continue
		}
		if err := downloadGeoData(ctx, client, url, path); err != nil {
			return fmt.Errorf("lite Geo 数据 %s 下载失败 (%s): %w；请检查网络后重试，或将 mihomo 官方文件放入 %s", name, url, err, directory)
		}
	}
	return nil
}

func downloadGeoData(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".geo-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	const maxSize = 128 * 1024 * 1024
	n, copyErr := io.Copy(file, io.LimitReader(resp.Body, maxSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n == 0 || n > maxSize {
		return fmt.Errorf("invalid Geo data size: %d", n)
	}
	return os.Rename(file.Name(), path)
}
