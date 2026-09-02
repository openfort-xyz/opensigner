package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// setupHandlerTest points the package's db at an in-memory sqlite instance
// with the production schema, so handler tests exercise real queries.
func setupHandlerTest(t *testing.T) {
	t.Helper()
	withTestKey(t)

	original := db
	tdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	for _, model := range []any{&Device{}, &Signer{}, &Account{}, &MigratedAccountData{}} {
		if err := tdb.AutoMigrate(model); err != nil {
			t.Fatalf("failed to migrate %T: %v", model, err)
		}
	}
	db = tdb
	t.Cleanup(func() { db = original })
}

func asUser(r *http.Request, userId, provider string) *http.Request {
	ctx := context.WithValue(r.Context(), fieldUserId, userId)
	ctx = context.WithValue(ctx, fieldAuthProvider, provider)
	return r.WithContext(ctx)
}

func seedAccount(t *testing.T, userId, provider, address string) (Account, Device) {
	t.Helper()
	signer := Signer{ID: uuid.NewString()}
	if err := db.Create(&signer).Error; err != nil {
		t.Fatalf("seed signer: %v", err)
	}
	device, err := newDevice("share-of-"+userId, signer.ID, true)
	if err != nil {
		t.Fatalf("seed device: %v", err)
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("seed device row: %v", err)
	}
	account := Account{
		ID:           uuid.NewString(),
		Address:      address,
		Username:     userId,
		ChainId:      80002,
		AuthProvider: provider,
		SignerId:     signer.ID,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return account, device
}

// Every query must filter on both the authenticated username and the auth
// provider; dropping either predicate is an IDOR. These tests pin that.
func TestGetDeviceIsScopedToOwner(t *testing.T) {
	setupHandlerTest(t)
	_, aliceDevice := seedAccount(t, "alice", "default", "0xaaa1")
	_, bobDevice := seedAccount(t, "bob", "default", "0xbbb1")

	h := apiMux()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, "/v1/devices/"+bobDevice.ID, nil), "alice", "default"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("alice fetching bob's device got %d, want 404", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, "/v1/devices/"+aliceDevice.ID, nil), "alice", "default"))
	if w.Code != http.StatusOK {
		t.Fatalf("alice fetching her own device got %d, want 200", w.Code)
	}
	var resp DeviceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Share != "share-of-alice" {
		t.Fatalf("got share %q, want the decrypted seeded share", resp.Share)
	}
}

func TestListAccountsIsScopedToUserAndProvider(t *testing.T) {
	setupHandlerTest(t)
	seedAccount(t, "alice", "default", "0xaaa2")
	seedAccount(t, "bob", "default", "0xbbb2")
	// Same username under a different provider is a different subject.
	seedAccount(t, "alice", "google", "0xccc2")

	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, "/v2/accounts", nil), "alice", "default"))
	if w.Code != http.StatusOK {
		t.Fatalf("list accounts got %d, want 200", w.Code)
	}
	var resp AccountListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 1 || resp.Data[0].Address != "0xaaa2" {
		t.Fatalf("alice@default sees %d account(s) %v, want only her own", resp.Total, resp.Data)
	}
}

func TestImportShareConflicts(t *testing.T) {
	setupHandlerTest(t)
	account, _ := seedAccount(t, "bob", "default", "0xheld")

	importReq := func() *http.Request {
		body := `{"address":"0xheld","share":"some-share","chainId":80002,"userId":"former-alice"}`
		r := httptest.NewRequest(http.MethodPost, "/v2/accounts/import-share", strings.NewReader(body))
		return asUser(r, "alice", "default")
	}

	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, importReq())
	if w.Code != http.StatusConflict {
		t.Fatalf("import to a held address got %d, want 409", w.Code)
	}

	// A soft-deleted holder still owns the row under the unique index, so it
	// must be reported as the same conflict, not an internal error.
	if err := db.Delete(&account).Error; err != nil {
		t.Fatalf("soft-delete account: %v", err)
	}
	w = httptest.NewRecorder()
	apiMux().ServeHTTP(w, importReq())
	if w.Code != http.StatusConflict {
		t.Fatalf("import to a soft-deleted address got %d, want 409", w.Code)
	}
}

func TestImportShareRequiresUserId(t *testing.T) {
	setupHandlerTest(t)
	body := `{"address":"0xnew","share":"some-share","chainId":80002}`
	r := httptest.NewRequest(http.MethodPost, "/v2/accounts/import-share", strings.NewReader(body))
	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, asUser(r, "alice", "default"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("import without userId got %d, want 400", w.Code)
	}
	var n int64
	db.Unscoped().Model(&Account{}).Where("address = ?", "0xnew").Count(&n)
	if n != 0 {
		t.Fatalf("import without userId persisted %d account row(s), want 0", n)
	}
}

// The unique index on address is global, so the pre-check must be too: a
// holder under another provider is a 409, not an index violation reported as 500.
func TestCreateDeviceV2ConflictsAcrossProviders(t *testing.T) {
	setupHandlerTest(t)
	seedAccount(t, "bob", "google", "0xheld")

	body := `{"address":"0xheld","share":"some-share","chainId":80002}`
	r := httptest.NewRequest(http.MethodPost, "/v2/devices/create", strings.NewReader(body))
	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, asUser(r, "alice", "default"))
	if w.Code != http.StatusConflict {
		t.Fatalf("create on an address held under another provider got %d, want 409", w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	setupHandlerTest(t)
	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, "/v2/devices/create", nil), "alice", "default"))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET on create got %d, want 405", w.Code)
	}
}

func TestBareDevicesPathRoutesToList(t *testing.T) {
	setupHandlerTest(t)
	seedAccount(t, "alice", "default", "0xaaa3")

	w := httptest.NewRecorder()
	apiMux().ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, "/v1/devices", nil), "alice", "default"))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/devices got %d, want the device list", w.Code)
	}
	var resp DeviceListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("got %d devices, want 1", resp.Total)
	}
}

// The composed chain must authenticate before it rate-limits: a flood of
// unauthenticated requests yields 401s, never a 429, because the limiter has
// no verified subject to charge.
func TestUnauthenticatedRequestsAreRejectedNotThrottled(t *testing.T) {
	setupHandlerTest(t)
	h := buildHandler()

	for i := 0; i < rateLimitBurst+10; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v2/accounts", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("request %d got %d, want 401", i+1, w.Code)
		}
	}
}
