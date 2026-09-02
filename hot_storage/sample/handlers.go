package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// contextKey is unexported so no other package can collide with these keys.
type contextKey string

const (
	contentTypeHeader = "Content-Type"
	contentTypeJSON   = "application/json"
	fieldDeviceId     = "deviceId"
	fieldAddress      = "address"
	actionRegister    = "REGISTER"
	actionRecover     = "RECOVER"

	errNotFound = "resource not found"
	errConflict = "resource already exists"

	fieldUserId       contextKey = "userId"
	fieldAuthProvider contextKey = "authProvider"
)

// authContext returns the authenticated subject placed in the request context
// by authMiddleware. ok is false when the request never passed through it.
func authContext(r *http.Request) (userId, authProvider string, ok bool) {
	userId, uok := r.Context().Value(fieldUserId).(string)
	authProvider, pok := r.Context().Value(fieldAuthProvider).(string)
	if !uok || !pok || userId == "" {
		return "", "", false
	}
	return userId, authProvider, true
}

// contentTypeMiddleware requires exactly application/json (parsed, not
// substring-matched) on requests with a body. text/plain and an absent
// Content-Type are CORS-safelisted, so accepting them would let state-changing
// requests skip the CORS preflight.
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get(contentTypeHeader))
			if err != nil || mediaType != contentTypeJSON {
				http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// apiMux holds the authenticated route table, separate from the middleware
// chain so tests can drive the same routes with an injected identity.
func apiMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/devices/init", handleInitDevice)
	mux.HandleFunc("/v1/devices/register", handleRegisterDevice)
	mux.HandleFunc("/v1/devices/{deviceId}", handleGetDevice)
	mux.HandleFunc("/v1/devices", handleGetDevices)

	mux.HandleFunc("/v2/devices/create", handleCreateDeviceV2)
	mux.HandleFunc("/v2/accounts", handleListAccountsV2)
	mux.HandleFunc("/v2/accounts/signer", handleGetSignerV2)
	mux.HandleFunc("/v2/devices/recover", handleRecoverDeviceV2)
	mux.HandleFunc("/v2/devices/register", handleRegisterDeviceV2)
	mux.HandleFunc("/v2/accounts/import-share", handleImportShare)
	mux.HandleFunc("/v2/accounts/migrated-data", handleGetMigratedAccountData)
	return mux
}

// buildHandler assembles the routes and middleware chain; split from
// listenAndServe so tests can exercise the exact composition that serves.
func buildHandler() http.Handler {
	// Order matters: the rate limiter keys on the authenticated user id, so it
	// sits inside authMiddleware.
	rl := newRateLimiter()
	go rl.sweep()
	handler := contentTypeMiddleware(authMiddleware(rateLimitMiddleware(rl, apiMux())))
	handler = corsMiddleware(handler)

	// Health endpoint outside auth middleware
	root := http.NewServeMux()
	root.HandleFunc("/health", handleHealth)
	root.Handle("/", handler)
	return root
}

func listenAndServe(addr string) {
	if err := http.ListenAndServe(addr, buildHandler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// newDevice mints a device id and encrypts the share bound to it, so a call
// site cannot store a share under a different id than the one it was bound to.
func newDevice(share, signerID string, isPrimary bool) (Device, error) {
	deviceID := uuid.NewString()
	encryptedShare, err := encryptShare(share, deviceID)
	if err != nil {
		return Device{}, err
	}
	return Device{
		ID:        deviceID,
		Share:     encryptedShare,
		IsPrimary: isPrimary,
		SignerId:  signerID,
	}, nil
}

// decryptDeviceShare decrypts a device's stored share, logging any failure: a
// decrypt error here is either key rotation gone wrong or a ciphertext moved
// between rows, and both need an audit trail.
func decryptDeviceShare(w http.ResponseWriter, device Device) (string, bool) {
	share, err := decryptShare(device.Share, device.ID)
	if err != nil {
		slog.Error("share decryption failed", "device_id", device.ID, "error", err)
		http.Error(w, "failed to decrypt share", http.StatusInternalServerError)
		return "", false
	}
	return share, true
}

func handleRegisterDeviceV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequestV2
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "username = ? AND id = ? AND auth_provider = ?", userId, req.Account, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	// This endpoint saves only "secondary" shares.
	device, err := newDevice(req.Share, account.SignerId, false)
	if err != nil {
		http.Error(w, "failed to encrypt share", http.StatusInternalServerError)
		return
	}
	if err := db.Create(&device).Error; err != nil {
		http.Error(w, "failed to register device", http.StatusInternalServerError)
		return
	}

	resp := EmbeddedResponse{
		Address:      account.Address,
		ChainID:      account.ChainId,
		DeviceID:     device.ID,
		Device:       device.ID,
		Account:      account.ID,
		OwnerAddress: account.Address,
		AccountType:  "Externally Owned Account",
		Signer:       account.SignerId,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleRecoverDeviceV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecoverEmbeddedRequestV2
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "username = ? AND id = ? AND auth_provider = ?", userId, req.Account, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	var device Device
	if err := db.First(&device, "signer_id = ? AND is_primary = true", account.SignerId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	decryptedShare, ok := decryptDeviceShare(w, device)
	if !ok {
		return
	}

	resp := RecoverResponseV2{
		Id:            device.ID,
		Account:       account.ID,
		SignerAddress: account.Address,
		Signer:        fmt.Sprintf("sig_%s", account.SignerId),
		Share:         decryptedShare,
		IsPrimary:     device.IsPrimary,
		User:          userId,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleListAccountsV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	var accounts []Account
	query := db.Where("username = ? AND auth_provider = ?", userId, authProvider)
	if err := query.Limit(limit).Find(&accounts).Error; err != nil {
		http.Error(w, "failed to select users' accounts", http.StatusInternalServerError)
		return
	}

	accountsResponse := make([]AccountResponse, 0)

	for _, acc := range accounts {
		accountsResponse = append(accountsResponse, AccountResponse{
			ID:       acc.ID,
			Address:  acc.Address,
			Username: acc.Username,
			ChainId:  acc.ChainId,
			SignerId: acc.SignerId,
		})
	}

	resp := AccountListResponse{
		Object: "list",
		URL:    "/v2/accounts",
		Data:   accountsResponse,
		Start:  0,
		End:    len(accountsResponse) - 1,
		Total:  len(accountsResponse),
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleGetSignerV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	address := r.URL.Query().Get(fieldAddress)
	if address == "" {
		http.Error(w, "missed address parameter", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "address = ? AND username = ? AND auth_provider = ?", address, userId, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	signer := GetSignerResponse{
		Id: account.SignerId,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(signer)
}

func handleCreateDeviceV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateEmbeddedRequestV2
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var resp EmbeddedResponse
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// Unscoped: the unique index on address is not soft-delete-aware, so a
		// soft-deleted holder must surface as a 409 here rather than as an
		// index violation at Create. Global, not per provider, to match the
		// index: a holder under another provider is the same conflict.
		var account Account
		if err := tx.Unscoped().First(&account, "address = ?", req.Address).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("database error")
			}
		}

		if account.ID != "" {
			return fmt.Errorf("conflict")
		}

		var signerUuid string
		if req.SignerUuid != nil {
			signerUuid = *req.SignerUuid
		} else {
			signerUuid = uuid.NewString()
		}

		signer := Signer{ID: signerUuid}
		if err := tx.Create(&signer).Error; err != nil {
			return fmt.Errorf("failed to create a signer")
		}

		device, err := newDevice(req.Share, signer.ID, true)
		if err != nil {
			return fmt.Errorf("failed to encrypt share")
		}
		if err := tx.Create(&device).Error; err != nil {
			return fmt.Errorf("failed to register device")
		}

		newAccount := Account{
			ID:           uuid.NewString(),
			Address:      req.Address,
			Username:     userId,
			ChainId:      req.ChainId,
			AuthProvider: authProvider,
			SignerId:     signer.ID,
		}
		if err := tx.Create(&newAccount).Error; err != nil {
			return fmt.Errorf("failed to create an account")
		}

		resp = EmbeddedResponse{
			Address:      req.Address,
			ChainID:      req.ChainId,
			DeviceID:     device.ID,
			Device:       device.ID,
			Account:      newAccount.ID,
			OwnerAddress: req.Address,
			AccountType:  "Externally Owned Account",
			Signer:       fmt.Sprintf("sig_%s", signer.ID),
		}
		return nil
	})

	if txErr != nil {
		if txErr.Error() == "conflict" {
			http.Error(w, errConflict, http.StatusConflict)
		} else {
			http.Error(w, txErr.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleInitDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InitEmbeddedRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	// Check if the user has a device for the given chainId, from the database through GORM.
	var account Account
	var nextAction NextAction
	if err := db.First(&account, "username = ? AND chain_id = ? AND auth_provider = ?", userId, req.ChainID, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			nextAction = NextAction{
				NextAction: actionRegister,
				Player:     userId,
				Embedded: &Embedded{
					ChainID: req.ChainID,
				},
			}
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	} else {
		var device Device
		err := db.First(&device, "signer_id = ? AND is_primary = true", account.SignerId).Error
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		decryptedShare, ok := decryptDeviceShare(w, device)
		if !ok {
			return
		}

		nextAction = NextAction{
			NextAction: actionRecover,
			Player:     userId,
			Embedded: &Embedded{
				ChainID: req.ChainID,
				Address: &account.Address,
				Share:   &decryptedShare,
			},
		}
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(nextAction)
}

// Devices will be registered to the username extracted from the JWT token.
func handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var req RegisterEmbeddedRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var resp EmbeddedResponse
	txErr := db.Transaction(func(tx *gorm.DB) error {
		isPrimary := false
		var account Account
		if err := tx.First(&account, "username = ? AND chain_id = ? AND address = ? AND auth_provider = ?", userId, req.ChainID, req.Address, authProvider).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("database error")
			}
			isPrimary = true
		}

		if !isPrimary {
			device, err := newDevice(req.Share, account.SignerId, false)
			if err != nil {
				return fmt.Errorf("failed to encrypt share")
			}
			if err := tx.Create(&device).Error; err != nil {
				return fmt.Errorf("failed to register device")
			}

			resp = EmbeddedResponse{
				Address:  req.Address,
				ChainID:  req.ChainID,
				DeviceID: device.ID,
				Device:   device.ID,
				Account:  account.ID,
			}
		} else {
			var signerUuid string
			if req.SignerUuid != nil {
				signerUuid = *req.SignerUuid
			} else {
				signerUuid = uuid.NewString()
			}

			signer := Signer{ID: signerUuid}
			if err := tx.Create(&signer).Error; err != nil {
				return fmt.Errorf("failed to save signer")
			}

			device, err := newDevice(req.Share, signer.ID, true)
			if err != nil {
				return fmt.Errorf("failed to encrypt share")
			}
			if err := tx.Create(&device).Error; err != nil {
				return fmt.Errorf("failed to register device")
			}

			account := Account{
				ID:           uuid.NewString(),
				Address:      req.Address,
				Username:     userId,
				ChainId:      req.ChainID,
				AuthProvider: authProvider,
				SignerId:     signer.ID,
			}
			if err := tx.Create(&account).Error; err != nil {
				return fmt.Errorf("failed to save account")
			}

			resp = EmbeddedResponse{
				Address:  req.Address,
				ChainID:  req.ChainID,
				DeviceID: device.ID,
				Device:   device.ID,
				Account:  account.ID,
				Signer:   fmt.Sprintf("sig_%s", signerUuid),
			}
		}
		return nil
	})

	if txErr != nil {
		http.Error(w, txErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleGetDevice(w http.ResponseWriter, r *http.Request) {
	// The {deviceId} pattern never matches an empty segment; bare /v1/devices
	// routes to handleGetDevices directly.
	deviceId := r.PathValue(fieldDeviceId)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if deviceId == "primary" {
		handleGetPrimaryDevice(w, r)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	// Verify device ownership through signer -> account relationship
	var device Device
	if err := db.Where("id = ? AND signer_id IN (SELECT signer_id FROM accounts WHERE username = ? AND auth_provider = ?)", deviceId, userId, authProvider).First(&device).Error; err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	decryptedShare, ok := decryptDeviceShare(w, device)
	if !ok {
		return
	}

	resp := DeviceResponse{
		ID:        device.ID,
		Object:    "device",
		CreatedAt: device.CreatedAt.Unix(),
		Share:     decryptedShare,
		IsPrimary: device.IsPrimary,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleGetPrimaryDevice(w http.ResponseWriter, r *http.Request) {
	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "username = ? AND auth_provider = ?", userId, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	var device Device
	if err := db.Where("signer_id = ? AND is_primary = ?", account.SignerId, true).First(&device).Error; err != nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}

	decryptedShare, ok := decryptDeviceShare(w, device)
	if !ok {
		return
	}

	resp := DeviceResponse{
		ID:        device.ID,
		Object:    "device",
		CreatedAt: device.CreatedAt.Unix(),
		Address:   account.Address,
		Share:     decryptedShare,
		IsPrimary: device.IsPrimary,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleGetDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleListDevices(w, r)
	case http.MethodPost:
		handleCreateDevice(w, r)
	default:
		http.NotFound(w, r)
	}
}

func handleListDevices(w http.ResponseWriter, r *http.Request) {
	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	var accounts []Account
	query := db.Where("username = ? AND auth_provider = ?", userId, authProvider)
	if err := query.Limit(limit).Find(&accounts).Error; err != nil {
		http.Error(w, "failed to select users' accounts", http.StatusInternalServerError)
		return
	}

	signerAccountMap := make(map[string]Account, 0)
	signerIds := make([]string, 0)

	for _, acc := range accounts {
		signerIds = append(signerIds, acc.SignerId)
		signerAccountMap[acc.SignerId] = acc
	}

	var devices []Device
	query = db.Where("signer_id IN ?", signerIds)
	if err := query.Limit(limit).Find(&devices).Error; err != nil {
		http.Error(w, "failed to list devices", http.StatusInternalServerError)
		return
	}

	deviceResponses := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		deviceResponses[i] = DeviceResponse{
			ID:        d.ID,
			Object:    "device",
			CreatedAt: d.CreatedAt.Unix(),
			Address:   signerAccountMap[d.SignerId].Address,
			IsPrimary: d.IsPrimary,
		}
	}

	resp := DeviceListResponse{
		Object: "list",
		URL:    "/v1/devices",
		Data:   deviceResponses,
		Start:  0,
		End:    len(deviceResponses) - 1,
		Total:  len(deviceResponses),
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleImportShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImportShareRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// userId is the former Openfort user id, stored as the passkey PRF seed.
	// Accepting it empty commits a row that recovery cannot use, and the
	// address is then held so a corrected import gets 409.
	if req.Address == "" || req.Share == "" || req.UserId == "" {
		http.Error(w, "address, share and userId are required", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	// Resolve the address that will be persisted before checking uniqueness, so
	// both operate on the same value. For a smart account the stored address is
	// ownerAddress rather than the request's address; checking one and storing the
	// other would let a caller claim an address another account already holds.
	accAddress := req.Address
	if req.SmartAccount != nil {
		if req.OwnerAddress == nil {
			http.Error(w, "ownerAddress is required for smart accounts", http.StatusBadRequest)
			return
		}
		accAddress = *req.OwnerAddress
	}

	var resp ImportShareResponse
	txErr := db.Transaction(func(tx *gorm.DB) error {
		// Uniqueness is enforced on the address as stored, matching accAddress
		// above. Unscoped: the unique index is not soft-delete-aware, so a
		// soft-deleted holder must be a 409, not an index violation at Create.
		var existing Account
		err := tx.Unscoped().First(&existing, "address = ?", accAddress).Error
		if err == nil {
			return fmt.Errorf("conflict")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("database error")
		}

		signerId := strings.TrimPrefix(req.SignerId, "sig_")
		if signerId == "" {
			signerId = uuid.NewString()
		}

		signer := Signer{ID: signerId}
		if err := tx.Create(&signer).Error; err != nil {
			return fmt.Errorf("failed to create signer")
		}

		device, err := newDevice(req.Share, signer.ID, true)
		if err != nil {
			return fmt.Errorf("failed to encrypt share")
		}
		if err := tx.Create(&device).Error; err != nil {
			return fmt.Errorf("failed to create device")
		}

		accountId := req.ID
		if accountId == "" {
			accountId = uuid.NewString()
		}

		newAccount := Account{
			ID:           accountId,
			Address:      accAddress,
			Username:     userId,
			ChainId:      req.ChainId,
			SignerId:     signer.ID,
			AuthProvider: authProvider,
		}
		if err := tx.Create(&newAccount).Error; err != nil {
			return fmt.Errorf("failed to create account")
		}

		newMigrateAccData := MigratedAccountData{
			ID:              accountId,
			Wallet:          req.Wallet,
			FormerOwnerUser: req.UserId,
		}
		if err := tx.Create(&newMigrateAccData).Error; err != nil {
			return fmt.Errorf("failed to create migrated account data")
		}

		resp = ImportShareResponse{
			ID:       newAccount.ID,
			Wallet:   req.Wallet,
			Address:  newAccount.Address,
			SignerId: signer.ID,
		}
		return nil
	})

	if txErr != nil {
		if txErr.Error() == "conflict" {
			http.Error(w, errConflict, http.StatusConflict)
		} else {
			http.Error(w, txErr.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func handleGetMigratedAccountData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	accountId := r.URL.Query().Get("accountId")
	if accountId == "" {
		http.Error(w, "accountId query parameter is required", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "id = ? AND username = ? AND auth_provider = ?", accountId, userId, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	var data MigratedAccountData
	if err := db.First(&data, "id = ?", accountId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusNotFound)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(data)
}

func handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId, authProvider, ok := authContext(r)
	if !ok {
		unauthorized(w)
		return
	}

	var account Account
	if err := db.First(&account, "id = ? AND username = ? AND auth_provider = ?", req.AccountId, userId, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	device, err := newDevice(req.Share, account.SignerId, false)
	if err != nil {
		http.Error(w, "failed to encrypt share", http.StatusInternalServerError)
		return
	}
	if err := db.Create(&device).Error; err != nil {
		http.Error(w, "failed to create device", http.StatusInternalServerError)
		return
	}

	resp := CreateDeviceResponse{
		ID:        device.ID,
		Object:    "device",
		CreatedAt: device.CreatedAt.Unix(),
		Address:   account.Address,
		IsPrimary: device.IsPrimary,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
