package services

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"tavily-proxy/server/internal/db"
	"tavily-proxy/server/internal/models"
)

func TestMasterKeyServiceFixedKeyOverridesDatabaseAndCannotReset(t *testing.T) {
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
	if err := database.WithContext(ctx).Create(&models.Setting{
		Key:   masterKeySettingKey,
		Value: "old-key",
	}).Error; err != nil {
		t.Fatalf("seed master key: %v", err)
	}

	const fixedKey = "fixed-key"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewMasterKeyService(database, logger, fixedKey)
	if err := service.LoadOrCreate(ctx); err != nil {
		t.Fatalf("load fixed master key: %v", err)
	}

	if got := service.Get(); got != fixedKey {
		t.Fatalf("loaded key = %q, want %q", got, fixedKey)
	}
	if !service.Authenticate(fixedKey) {
		t.Fatal("fixed key did not authenticate")
	}

	resetKey, err := service.Reset(ctx)
	if err != nil {
		t.Fatalf("reset fixed master key: %v", err)
	}
	if resetKey != fixedKey || service.Get() != fixedKey {
		t.Fatalf("fixed key changed after reset: reset=%q current=%q", resetKey, service.Get())
	}

	var setting models.Setting
	if err := database.WithContext(ctx).First(&setting, "key = ?", masterKeySettingKey).Error; err != nil {
		t.Fatalf("read stored master key: %v", err)
	}
	if setting.Value != fixedKey {
		t.Fatalf("stored key = %q, want %q", setting.Value, fixedKey)
	}
}
