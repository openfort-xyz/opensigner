package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	contentTypeHeader = "Content-Type"
	contentTypeJSON   = "application/json"
	fieldUserId       = "userId"
	fieldAuthProvider = "authProvider"
	fieldDeviceId     = "deviceId"
	fieldAddress      = "address"
	actionRegister    = "REGISTER"
	actionRecover     = "RECOVER"

	errNotFound = "resource not found"
	errConflict = "resource already exists"
)

// contentTypeMiddleware validates that POST/PUT/PATCH requests have a JSON Content-Type.
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			ct := r.Header.Get(contentTypeHeader)
			if ct != "" && !strings.Contains(ct, "application/json") {
				http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func listenAndServe(addr string) {
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

	handler := contentTypeMiddleware(authMiddleware(mux))
	handler = corsMiddleware(handler)

	// Health endpoint outside auth middleware
	root := http.NewServeMux()
	root.HandleFunc("/health", handleHealth)
	root.Handle("/", handler)

	if err := http.ListenAndServe(addr, root); err != nil {
		log.Fatalf("server failed: %v", err)
	}
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

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

	encryptedShare, err := encryptShare(req.Share)
	if err != nil {
		http.Error(w, "failed to encrypt share", http.StatusInternalServerError)
		return
	}

	device := Device{
		ID:        uuid.NewString(),
		Share:     encryptedShare,
		IsPrimary: false, // with this endpoint we save only "secondary" shares
		SignerId:  account.SignerId,
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

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

	decryptedShare, err := decryptShare(device.Share)
	if err != nil {
		http.Error(w, "failed to decrypt share", http.StatusInternalServerError)
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

	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

	var resp EmbeddedResponse
	txErr := db.Transaction(func(tx *gorm.DB) error {
		var account Account
		if err := tx.First(&account, "address = ? AND auth_provider = ?", req.Address, authProvider).Error; err != nil {
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

		encryptedShare, err := encryptShare(req.Share)
		if err != nil {
			return fmt.Errorf("failed to encrypt share")
		}

		device := Device{
			ID:        uuid.NewString(),
			Share:     encryptedShare,
			IsPrimary: true,
			SignerId:  signer.ID,
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

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

		decryptedShare, err := decryptShare(device.Share)
		if err != nil {
			http.Error(w, "failed to decrypt share", http.StatusInternalServerError)
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

	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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
			encryptedShare, err := encryptShare(req.Share)
			if err != nil {
				return fmt.Errorf("failed to encrypt share")
			}

			device := Device{
				ID:        uuid.NewString(),
				Share:     encryptedShare,
				IsPrimary: false,
				SignerId:  account.SignerId,
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

			encryptedShare, err := encryptShare(req.Share)
			if err != nil {
				return fmt.Errorf("failed to encrypt share")
			}

			device := Device{
				ID:        uuid.NewString(),
				Share:     encryptedShare,
				IsPrimary: true,
				SignerId:  signer.ID,
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
	deviceId := r.PathValue(fieldDeviceId)
	if deviceId == "" {
		handleGetDevices(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if deviceId == "primary" {
		handleGetPrimaryDevice(w, r)
		return
	}

	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

	// Verify device ownership through signer -> account relationship
	var device Device
	if err := db.Where("id = ? AND signer_id IN (SELECT signer_id FROM accounts WHERE username = ? AND auth_provider = ?)", deviceId, userId, authProvider).First(&device).Error; err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	decryptedShare, err := decryptShare(device.Share)
	if err != nil {
		http.Error(w, "failed to decrypt share", http.StatusInternalServerError)
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
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

	decryptedShare, err := decryptShare(device.Share)
	if err != nil {
		http.Error(w, "failed to decrypt share", http.StatusInternalServerError)
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
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

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

func handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

	var account Account
	if err := db.First(&account, "id = ? AND username = ? AND auth_provider = ?", req.AccountId, userId, authProvider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, errNotFound, http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	encryptedShare, err := encryptShare(req.Share)
	if err != nil {
		http.Error(w, "failed to encrypt share", http.StatusInternalServerError)
		return
	}

	device := Device{
		ID:        uuid.NewString(),
		Share:     encryptedShare,
		IsPrimary: false,
		SignerId:  account.SignerId,
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
