package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tavily-proxy/server/internal/config"
	"tavily-proxy/server/internal/db"
	"tavily-proxy/server/internal/services"
)

const testDesktopAccessKey = "desktop-search-only-test-key"

func newDesktopAccessTestRouter(t *testing.T, upstream http.HandlerFunc) (http.Handler, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	upstreamServer := httptest.NewServer(upstream)
	t.Cleanup(upstreamServer.Close)

	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	master := services.NewMasterKeyService(database, logger, "test-master-key")
	if err := master.LoadOrCreate(ctx); err != nil {
		t.Fatalf("master key init: %v", err)
	}

	keys := services.NewKeyService(database, logger)
	if _, err := keys.Create(ctx, "tvly-pool-test-key", "pool", 1000); err != nil {
		t.Fatalf("create key: %v", err)
	}
	proxy := services.NewTavilyProxy(upstreamServer.URL, 5*time.Second, keys, nil, nil, logger)

	return NewRouter(Dependencies{
		Config:           config.Config{DesktopAccessKey: testDesktopAccessKey},
		MasterKeyService: master,
		TavilyProxy:      proxy,
	}), master.Get()
}

func TestDesktopAccessKeyAllowsOnlySearchAndExtract(t *testing.T) {
	t.Parallel()

	upstreamPaths := make(chan string, 2)
	router, _ := newDesktopAccessTestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamPaths <- r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "search", path: "/search", body: `{"query":"hello"}`},
		{name: "extract", path: "/extract", body: `{"urls":["https://example.com"]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+testDesktopAccessKey)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("unexpected status: got %d want %d (body=%q)", w.Code, http.StatusOK, w.Body.String())
			}
			if got := <-upstreamPaths; got != tc.path {
				t.Fatalf("unexpected upstream path: got %q want %q", got, tc.path)
			}
		})
	}
}

func TestDesktopAccessKeyIsStrippedFromBodyAndQuery(t *testing.T) {
	t.Parallel()

	router, _ := newDesktopAccessTestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("api_key"); got != "" {
			t.Fatalf("api_key leaked to upstream query: %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "desktop" {
			t.Fatalf("unexpected source query: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("upstream body json: %v", err)
		}
		if _, ok := payload["api_key"]; ok {
			t.Fatalf("api_key leaked to upstream body: %s", body)
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
	})

	body := []byte(`{"query":"hello","api_key":"` + testDesktopAccessKey + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/search?api_key="+testDesktopAccessKey+"&source=desktop", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d (body=%q)", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestDesktopAccessKeyRejectsAllOtherRoutes(t *testing.T) {
	t.Parallel()

	router, _ := newDesktopAccessTestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("restricted route must not reach upstream: %s %s", r.Method, r.URL.Path)
	})

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/usage"},
		{method: http.MethodPost, path: "/search/"},
		{method: http.MethodGet, path: "/search"},
		{method: http.MethodPost, path: "/crawl"},
		{method: http.MethodPost, path: "/map"},
		{method: http.MethodPost, path: "/mcp"},
		{method: http.MethodGet, path: "/api/keys"},
		{method: http.MethodGet, path: "/api/settings/master-key"},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+testDesktopAccessKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("unexpected status: got %d want %d (body=%q)", w.Code, http.StatusUnauthorized, w.Body.String())
			}
		})
	}
}

func TestMasterKeyStillProxiesOtherTavilyRoutes(t *testing.T) {
	t.Parallel()

	router, masterKey := newDesktopAccessTestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/usage" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"key":{"usage":0,"limit":1000}}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	req.Header.Set("Authorization", "Bearer "+masterKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d (body=%q)", w.Code, http.StatusOK, w.Body.String())
	}
}
