package accesstoken_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/accesstoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

var testSecret = []byte("test-secret-at-least-32-bytes-long!")

func TestIssueParse_RoundTrip(t *testing.T) {
	userID, orgID, memID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	raw, err := accesstoken.Issue(testSecret, time.Minute, userID, orgID, memID, tenant.RoleOwner)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	claims, err := accesstoken.Parse(testSecret, raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != userID || claims.OrganizationID != orgID || claims.MembershipID != memID {
		t.Errorf("claims mismatch: %+v", claims)
	}
	if claims.Role != tenant.RoleOwner {
		t.Errorf("expected role owner, got %q", claims.Role)
	}
}

func TestParse_Expired(t *testing.T) {
	userID, orgID, memID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	raw, err := accesstoken.Issue(testSecret, -time.Minute, userID, orgID, memID, tenant.RoleOwner)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := accesstoken.Parse(testSecret, raw); err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParse_WrongSecret(t *testing.T) {
	userID, orgID, memID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	raw, err := accesstoken.Issue(testSecret, time.Minute, userID, orgID, memID, tenant.RoleOwner)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := accesstoken.Parse([]byte("a-completely-different-secret-32b"), raw); err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParse_Garbage(t *testing.T) {
	if _, err := accesstoken.Parse(testSecret, "not-a-jwt"); err == nil {
		t.Fatal("expected error for garbage input, got nil")
	}
}
