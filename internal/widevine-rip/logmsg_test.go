package widevinerip

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amdl/internal/i18n"
	"amdl/internal/logx"
)

// drmLogKeys lists the catalog keys this package logs with. Keep it in sync
// with the logx.*T calls in rip.go and download.go.
var drmLogKeys = []string{
	"net.drm.song_downloaded",
	"net.drm.decrypt_failed",
	"net.drm.decrypted",
	"net.drm.stream_downloaded",
	"net.drm.stream_clear",
	"net.drm.stream_decrypted",
}

func TestDRMLogKeysAreRegistered(t *testing.T) {
	for _, key := range drmLogKeys {
		if !i18n.Has(key) {
			t.Errorf("catalog 缺少 %s", key)
		}
	}
}

// TestDownloadStreamClearLogsThroughLogx checks the clear-stream path reports
// through logx: level marker plus translated body, no leftover English text.
func TestDownloadStreamClearLogsThroughLogx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("seg"))
	}))
	defer server.Close()

	var captured bytes.Buffer
	logx.SetOutput(&captured)
	defer logx.SetOutput(os.Stdout)
	defer i18n.Set(string(i18n.Default))

	i18n.Set("en")
	outputPath := filepath.Join(t.TempDir(), "clear.mp4")
	stream := Stream{URLs: []string{server.URL + "/init.mp4", server.URL + "/seg0.m4s"}}
	if err := DownloadStream(context.Background(), stream, outputPath); err != nil {
		t.Fatal(err)
	}

	got := captured.String()
	for _, want := range []string{
		logx.MarkerOK + " " + i18n.T("net.drm.stream_downloaded"),
		logx.MarkerDetail + " " + i18n.T("net.drm.stream_clear"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("日志缺少 %q，实际输出:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Stream is not encrypted") {
		t.Errorf("仍有未翻译的英文文案:\n%s", got)
	}
}
