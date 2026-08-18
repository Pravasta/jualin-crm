package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db/dbtest"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

func TestRepository_CreateAndFindByEmail(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := user.New(pool)

	id := uuid.Must(uuid.NewV7())
	created, err := repo.Create(ctx, id, "Budi@Example.com", "hash", "Budi Santoso")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.Email != "budi@example.com" {
		t.Errorf("expected email to be normalized to lowercase, got %q", created.Email)
	}

	// Lookup with a different case must still find the same user — email
	// is stored and compared lowercase.
	found, err := repo.FindByEmail(ctx, "BUDI@example.com")
	if err != nil {
		t.Fatalf("find by email failed: %v", err)
	}
	if found.ID != id {
		t.Errorf("expected to find the same user by email regardless of case, got id %s", found.ID)
	}
}

func TestRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := user.New(pool)

	id := uuid.Must(uuid.NewV7())
	if _, err := repo.Create(ctx, id, "find-by-id@example.com", "hash", "Test User"); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	found, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("find by id failed: %v", err)
	}
	if found.FullName != "Test User" {
		t.Errorf("expected full name 'Test User', got %q", found.FullName)
	}
}

func TestRepository_FindByEmail_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.NewPool(t)
	repo := user.New(pool)

	_, err := repo.FindByEmail(ctx, "nobody@example.com")
	if !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("expected httpx.ErrNotFound for an unknown email, got: %v", err)
	}
}
