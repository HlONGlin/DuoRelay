// The downloader uses only targets with pinned binary and dependency checksums.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type lockFile struct {
	Singbox struct {
		Version string            `yaml:"version"`
		SHA256  map[string]string `yaml:"sha256"`
	} `yaml:"singbox"`
	Mihomo struct {
		Version string            `yaml:"version"`
		SHA256  map[string]string `yaml:"sha256"`
	} `yaml:"mihomo"`
	Cronet struct {
		LinuxAMD64 struct {
			URL    string `yaml:"url"`
			SHA256 string `yaml:"sha256"`
		} `yaml:"linux-amd64"`
		LinuxARM64 struct {
			URL    string `yaml:"url"`
			SHA256 string `yaml:"sha256"`
		} `yaml:"linux-arm64"`
	} `yaml:"cronet"`
}

func main() {
	arch := flag.String("arch", "", "target architecture")
	out := flag.String("out", "", "output directory")
	variant := flag.String("variant", "full", "full or lite")
	flag.Parse()
	if err := download(*arch, *out, *variant); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func download(arch, out, variant string) error {
	if variant != "full" && variant != "lite" {
		return fmt.Errorf("unknown variant %q: use full or lite", variant)
	}
	if out == "" || (arch != "amd64" && arch != "arm64") {
		return fmt.Errorf("unsupported target %s/linux/%s: full and lite support amd64 and arm64; output directory is required", variant, arch)
	}
	data, err := os.ReadFile("kernels.lock")
	if err != nil {
		return err
	}
	var lock lockFile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return err
	}
	key := "linux-" + arch
	cronet := lock.Cronet.LinuxAMD64
	if arch == "arm64" {
		cronet = lock.Cronet.LinuxARM64
	}
	if lock.Mihomo.Version == "" || len(lock.Mihomo.SHA256[key]) != 64 || (variant == "full" && (lock.Singbox.Version == "" || len(lock.Singbox.SHA256[key]) != 64 || cronet.URL == "" || len(cronet.SHA256) != 64)) {
		return fmt.Errorf("incomplete kernel lock for %s", key)
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	get := func(url string) (io.ReadCloser, error) {
		response, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("download %s: %s", url, response.Status)
		}
		return response.Body, nil
	}
	write := func(name, expected string, source io.Reader) error {
		if len(expected) != 64 {
			return fmt.Errorf("missing SHA-256 for %s", name)
		}
		file, err := os.OpenFile(filepath.Join(out, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(file, hash), source)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(expected) {
			return fmt.Errorf("SHA-256 mismatch for %s", name)
		}
		return nil
	}
	if variant == "full" {
		singURL := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-linux-%s.tar.gz", lock.Singbox.Version, lock.Singbox.Version, arch)
		body, err := get(singURL)
		if err != nil {
			return err
		}
		defer body.Close()
		gz, err := gzip.NewReader(body)
		if err != nil {
			return err
		}
		defer gz.Close()
		tarReader := tar.NewReader(gz)
		found := false
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "sing-box" {
				if found {
					return fmt.Errorf("duplicate sing-box binary")
				}
				if err := write("sing-box", lock.Singbox.SHA256[key], tarReader); err != nil {
					return err
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("sing-box binary missing from archive")
		}
	}
	mihomoURL := fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/v%s/mihomo-linux-%s-v%s.gz", lock.Mihomo.Version, arch, lock.Mihomo.Version)
	body2, err := get(mihomoURL)
	if err != nil {
		return err
	}
	defer body2.Close()
	gz2, err := gzip.NewReader(body2)
	if err != nil {
		return err
	}
	defer gz2.Close()
	if err := write("mihomo", lock.Mihomo.SHA256[key], gz2); err != nil {
		return err
	}
	if variant == "lite" {
		return nil
	}
	body3, err := get(cronet.URL)
	if err != nil {
		return err
	}
	defer body3.Close()
	return write("libcronet.so", cronet.SHA256, body3)
}
