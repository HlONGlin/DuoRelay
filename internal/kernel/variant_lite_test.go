//go:build lite

package kernel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/variant"
)

func TestLiteRegistryAndLifecycle(t *testing.T) {
	if !reflect.DeepEqual(variant.Kernels(), []string{"mihomo"}) {
		t.Fatal(variant.Kernels())
	}
	m := testManager(t)
	if _, found := m.Status()["singbox"]; found {
		t.Fatal("sing-box registered in lite")
	}
	if err := m.StartSingBox("missing"); err == nil {
		t.Fatal("sing-box accepted")
	}
	if err := m.SetMode(ModeCoexist); err == nil {
		t.Fatal("coexist accepted")
	}
	if err := m.Validate("sing-box", "missing"); err == nil {
		t.Fatal("sing-box validator accepted")
	}
	path := writeConfig(t, "mihomo", nil)
	if err := m.StartMihomo(path); err != nil {
		t.Fatal(err)
	}
	if kernelStatus(m, "mihomo")["running"] != true {
		t.Fatal(m.Status())
	}
	if err := m.Validate("mihomo", writeConfig(t, "mihomo", map[string]any{"invalid": true})); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	if kernelStatus(m, "mihomo")["running"] != true {
		t.Fatal("validation stopped running kernel")
	}
	if err := m.StopAll(); err != nil {
		t.Fatal(err)
	}
}

func TestLiteGeoCacheAndFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, "geo fixture") }))
	defer server.Close()
	dir := t.TempDir()
	assets := map[string]string{"geoip.metadb": server.URL}
	for i := 0; i < 2; i++ {
		if err := prepareGeoData(context.Background(), server.Client(), dir, assets); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("cache downloaded %d times", calls)
	}
	for _, code := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusOK} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
			defer bad.Close()
			dir := t.TempDir()
			err := prepareGeoData(context.Background(), bad.Client(), dir, map[string]string{"GeoSite.dat": bad.URL})
			if err == nil || !strings.Contains(err.Error(), "下载失败") {
				t.Fatalf("failure hidden: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "GeoSite.dat")); !os.IsNotExist(err) {
				t.Fatal("partial file cached")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("download temporary files leaked")
			}
		})
	}
}
