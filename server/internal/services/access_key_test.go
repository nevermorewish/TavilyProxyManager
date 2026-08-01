package services

import (
	"context"
	"path/filepath"
	"testing"

	"tavily-proxy/server/internal/db"
)

func TestAccessKeyServiceLoadCreateDeleteAndAuthenticate(t *testing.T) {
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
	service := NewAccessKeyService(database)
	const desktopKey = "desktop-access-key"
	if err := service.Load(ctx, desktopKey); err != nil {
		t.Fatalf("load access keys: %v", err)
	}

	items, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list access keys: %v", err)
	}
	if len(items) != 1 || items[0].Name != defaultAccessKeyName || items[0].Key != desktopKey {
		t.Fatalf("unexpected default access key: %+v", items)
	}
	if service.Authenticate(desktopKey) {
		t.Fatal("restricted desktop key unexpectedly authenticated as a full access key")
	}
	if !service.AuthenticateRestricted(desktopKey) {
		t.Fatal("desktop access key did not authenticate for restricted access")
	}

	created, err := service.Create(ctx, "Automation")
	if err != nil {
		t.Fatalf("create access key: %v", err)
	}
	if created.Key == "" || !service.Authenticate(created.Key) {
		t.Fatal("created access key did not authenticate")
	}

	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete access key: %v", err)
	}
	if service.Authenticate(created.Key) {
		t.Fatal("deleted access key still authenticates")
	}
	if err := service.Delete(ctx, items[0].ID); err != nil {
		t.Fatalf("delete default access key: %v", err)
	}
	if err := service.Load(ctx, "replacement-desktop-key"); err != nil {
		t.Fatalf("reload after deleting all keys: %v", err)
	}
	items, err = service.List(ctx)
	if err != nil {
		t.Fatalf("list after reload: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("deleted default key was recreated: %+v", items)
	}
}

func TestAccessKeyServiceLoadDoesNotOverwriteExistingKeys(t *testing.T) {
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
	service := NewAccessKeyService(database)
	if err := service.Load(ctx, "first-desktop-key"); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if err := service.Load(ctx, "replacement-desktop-key"); err != nil {
		t.Fatalf("second load: %v", err)
	}
	if !service.AuthenticateRestricted("first-desktop-key") {
		t.Fatal("initial key was overwritten")
	}
	if service.Authenticate("replacement-desktop-key") {
		t.Fatal("replacement environment key was unexpectedly imported")
	}
}
