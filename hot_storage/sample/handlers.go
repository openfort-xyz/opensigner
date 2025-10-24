package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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
)

func listenAndServe(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/v1/devices/init", handleInitDevice)
	mux.HandleFunc("/v1/devices/register", handleRegisterDevice)
	mux.HandleFunc("/v1/devices/switch-chain", handleSwitchChain)
	mux.HandleFunc("/v1/devices/{deviceId}", handleGetDevice)
	mux.HandleFunc("/v1/devices", handleGetDevices)

	mux.HandleFunc("/v2/devices/create", handleCreateDeviceV2)
	mux.HandleFunc("/v2/accounts", handleListAccountsV2)
	mux.HandleFunc("/v2/accounts/signer", handleGetSignerV2)
	mux.HandleFunc("/v2/accounts/switch-chain", handleSwitchChainV2)
	mux.HandleFunc("/v2/devices/recover", handleRecoverDeviceV2)
	mux.HandleFunc("/v2/devices/register", handleRegisterDeviceV2)

	handler := authMiddleware(mux)
	handler = corsMiddleware(handler)
	http.ListenAndServe(addr, handler)
}

func handleRegisterDeviceV2(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequestV2
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	authProvider := r.Header.Get(headerAuthProvider)
	if authProvider == "" {
		authProvider = auhtProviderDefault
	}

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		io.WriteString(w, "Missing field: userId")
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)

	var account Account
	if err := db.First(&account, "username = ? AND id = ?", userId, req.Account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "account doesn't exist", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	device := Device{
		ID:        uuid.NewString(),
		Share:     req.Share,
		IsPrimary: false, // with this endpoint we save only "secondary" shares
		SignerId:  account.SignerId,
	}
	if err := db.Create(&device).Error; err != nil {
		http.Error(w, "failed to register device", http.StatusInternalServerError)
		return
	}

	resp := EmbeddedResponse{
		Share:        req.Share,
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
	var req RecoverEmbeddedRequestV2
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	authProvider := r.Header.Get(headerAuthProvider)
	if authProvider == "" {
		authProvider = auhtProviderDefault
	}

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		io.WriteString(w, "Missing field: userId")
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)

	var account Account
	if err := db.First(&account, "username = ? AND id = ?", userId, req.Account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "account doesn't exist", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	var device Device
	if err := db.First(&device, "signer_id = ? AND is_primary = true", account.SignerId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "primary device was not found", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	resp := RecoverResponseV2{
		Id:            device.ID,
		Account:       account.ID,
		SignerAddress: account.Address,
		Signer:        fmt.Sprintf("sig_%s", account.SignerId),
		Share:         device.Share,
		IsPrimary:     device.IsPrimary,
		User:          userId,
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleListAccountsV2(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Context().Value(fieldAuthProvider).(string)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
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

func handleSwitchChainV2(w http.ResponseWriter, r *http.Request) {
	var req SwitchChainQueriesV2
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		io.WriteString(w, "Missing field: userId")
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)

	var account Account
	if err := db.First(&account, "id = ?", req.Account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "account doesn't exist", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	var device Device
	if err := db.First(&device, "signer_id = ?", account.SignerId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "device not found for account", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	newAccount := Account{
		ID:           uuid.NewString(),
		Address:      account.Address,
		Username:     userId,
		ChainId:      req.ChainId,
		AuthProvider: account.AuthProvider,
		SignerId:     account.SignerId,
	}
	if err := db.Create(&newAccount).Error; err != nil {
		http.Error(w, "failed to save new account", http.StatusInternalServerError)
		return
	}

	timestamp := 1532009163
	t := time.Unix(int64(timestamp), 0)

	response := SwitchChainResponseV2{
		Id:           newAccount.ID,
		User:         userId,
		AccountType:  "Externally Owned Account",
		Address:      account.Address,
		OwnerAddress: account.Address,
		ChainType:    "EVM",
		ChainId:      req.ChainId,
		CreatedAt:    time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix(),
		UpdatedAt:    time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix(),
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(response)
}

func handleGetSignerV2(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get(fieldAddress)
	if address == "" {
		http.Error(w, "missed address parameter", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var account Account
	if err := db.Find(&account, "address = ?", address).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "account by address not found", http.StatusBadRequest)
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
	var req CreateEmbeddedRequestV2
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	authProvider := r.Header.Get(headerAuthProvider)
	if authProvider == "" {
		authProvider = auhtProviderDefault
	}

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		io.WriteString(w, "Missing field: userId")
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)

	var account Account
	if err := db.First(&account, "address = ?", req.Address).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	if account.ID != "" {
		http.Error(w, "account already exist", http.StatusBadRequest)
		return
	}

	var signerUuid string

	if req.SignerUuid != nil {
		signerUuid = *req.SignerUuid
	} else {
		signerUuid = uuid.NewString()
	}

	signer := Signer{ID: signerUuid}
	if err := db.Create(&signer).Error; err != nil {
		http.Error(w, "failed to create a signer", http.StatusInternalServerError)
		return
	}

	device := Device{
		ID:        uuid.NewString(),
		Share:     req.Share,
		IsPrimary: true,
		SignerId:  signer.ID,
	}
	if err := db.Create(&device).Error; err != nil {
		http.Error(w, "failed to register device", http.StatusInternalServerError)
		return
	}

	newAccount := Account{
		ID:           uuid.NewString(),
		Address:      req.Address,
		Username:     userId,
		ChainId:      req.ChainId,
		AuthProvider: authProvider,
		SignerId:     signer.ID,
	}
	if err := db.Create(&newAccount).Error; err != nil {
		http.Error(w, "failed to create an account", http.StatusInternalServerError)
		return
	}

	resp := EmbeddedResponse{
		Share:        req.Share,
		Address:      req.Address,
		ChainID:      req.ChainId,
		DeviceID:     device.ID,
		Device:       device.ID,
		Account:      newAccount.ID,
		OwnerAddress: req.Address,
		AccountType:  "Externally Owned Account",
		Signer:       fmt.Sprintf("sig_%s", signer.ID),
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleInitDevice(w http.ResponseWriter, r *http.Request) {
	var req InitEmbeddedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userIdAny := r.Context().Value(fieldUserId)
	if userIdAny == nil {
		io.WriteString(w, "Missing field: userId")
		unauthorized(w)
		return
	}
	userId := r.Context().Value(fieldUserId).(string)

	// Check if the user has a device for the given chainId, from the database through GORM.
	var account Account
	var nextAction NextAction
	if err := db.First(&account, "username = ? AND chain_id = ?", userId, req.ChainID).Error; err != nil {
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
			http.Error(w, "no device found for signer but account is created", http.StatusInternalServerError)
			return
		}

		nextAction = NextAction{
			NextAction: actionRecover,
			Player:     userId,
			Embedded: &Embedded{
				ChainID: req.ChainID,
				Address: &account.Address,
				Share:   &device.Share,
			},
		}
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(nextAction)
}

// Devices will be registered to the username extracted from the JWT token.
func handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(fieldUserId).(string)
	authProvider := r.Header.Get(headerAuthProvider)
	if authProvider == "" {
		authProvider = auhtProviderDefault
	}

	var req RegisterEmbeddedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	isPrimary := false
	var account Account
	if err := db.First(&account, "username = ? AND chain_id = ? AND address = ?", userId, req.ChainID, req.Address).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		isPrimary = true
	}

	if !isPrimary {
		device := Device{
			ID:        uuid.NewString(),
			Share:     req.Share,
			IsPrimary: isPrimary,
			SignerId:  account.SignerId,
		}
		if err := db.Create(&device).Error; err != nil {
			http.Error(w, "failed to register device", http.StatusInternalServerError)
			return
		}

		resp := EmbeddedResponse{
			Share:    req.Share,
			Address:  req.Address,
			ChainID:  req.ChainID,
			DeviceID: device.ID,
			Device:   device.ID,
			Account:  account.ID,
		}

		w.Header().Set(contentTypeHeader, contentTypeJSON)
		json.NewEncoder(w).Encode(resp)
	} else {
		var signerUuid string

		if req.SignerUuid != nil {
			signerUuid = *req.SignerUuid
		} else {
			signerUuid = uuid.NewString()
		}

		signer := Signer{ID: signerUuid}
		if err := db.Create(&signer).Error; err != nil {
			http.Error(w, "failed to save signer", http.StatusInternalServerError)
			return
		}

		device := Device{
			ID:        uuid.NewString(),
			Share:     req.Share,
			IsPrimary: isPrimary,
			SignerId:  signer.ID,
		}
		if err := db.Create(&device).Error; err != nil {
			http.Error(w, "failed to register device", http.StatusInternalServerError)
			return
		}

		account := Account{
			ID:           uuid.NewString(),
			Address:      req.Address,
			Username:     userId,
			ChainId:      req.ChainID,
			AuthProvider: authProvider,
			SignerId:     signer.ID,
		}
		if err := db.Create(&account).Error; err != nil {
			http.Error(w, "failed to save account", http.StatusInternalServerError)
			return
		}

		resp := EmbeddedResponse{
			Share:    req.Share,
			Address:  req.Address,
			ChainID:  req.ChainID,
			DeviceID: device.ID,
			Device:   device.ID,
			Account:  account.ID,
			Signer:   fmt.Sprintf("sig_%s", signerUuid),
		}

		w.Header().Set(contentTypeHeader, contentTypeJSON)
		json.NewEncoder(w).Encode(resp)
	}
}

func handleSwitchChain(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(fieldUserId).(string)

	var req SwitchChainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var device Device
	if err := db.Where("id = ? AND username = ?", req.DeviceID, userId).First(&device).Error; err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	if err := db.Model(&device).Update("chain_id", req.ChainID).Error; err != nil {
		http.Error(w, "failed to update device", http.StatusInternalServerError)
		return
	}

	resp := EmbeddedResponse{
		Share:    device.Share,
		ChainID:  req.ChainID,
		DeviceID: device.ID,
		// Address:, there may be a few account relations to one device
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

	var device Device
	if err := db.Where("id = ?", deviceId).First(&device).Error; err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	resp := DeviceResponse{
		ID:        device.ID,
		Object:    "device",
		CreatedAt: device.CreatedAt.Unix(),
		Share:     device.Share,
		IsPrimary: device.IsPrimary,
		// Address:, there may be a few accounts relation to one device
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

func handleGetPrimaryDevice(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(fieldUserId).(string)

	var account Account
	if err := db.First(&account, "username = ?", userId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "account by username not found", http.StatusBadRequest)
			return
		} else {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}

	var device Device
	if err := db.Where("signer_id = ? AND is_primary = ?", account.SignerId, true).First(&device).Error; err != nil {
		http.Error(w, "primary device not found", http.StatusNotFound)
		return
	}

	resp := DeviceResponse{
		ID:        device.ID,
		Object:    "device",
		CreatedAt: device.CreatedAt.Unix(),
		Address:   account.Address,
		Share:     device.Share,
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
	if limit == 0 {
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
			Share:     d.Share,
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var account Account
	err := db.Find(&account, "id = ?", req.AccountId).Error
	if err != nil {
		http.Error(w, "account with such ID wasn't found", http.StatusBadRequest)
		return
	}

	device := Device{
		ID:        uuid.NewString(),
		Share:     req.Share,
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
		Share:     device.Share,
		IsPrimary: device.IsPrimary,
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}
