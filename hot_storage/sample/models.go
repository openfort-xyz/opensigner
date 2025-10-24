package main

import (
	"gorm.io/gorm"
)

type InitEmbeddedRequest struct {
	ChainID int64 `json:"chainId"`
}

type NextAction struct {
	NextAction string    `json:"nextAction"`
	Player     string    `json:"player"` // TODO: rename to user
	Embedded   *Embedded `json:"embedded,omitempty"`
}

type Embedded struct {
	Share        *string `json:"share,omitempty"`
	OwnerAddress *string `json:"ownerAddress,omitempty"`
	Address      *string `json:"address,omitempty"`
	ChainID      int64   `json:"chainId"`
	DeviceID     *string `json:"deviceId,omitempty"`
}

type RegisterEmbeddedRequest struct {
	ChainID    int64   `json:"chainId"`
	Address    string  `json:"address"`
	Share      string  `json:"share"`
	SignerUuid *string `json:"signerUuid"`
}

type SwitchChainRequest struct {
	ChainID  int64  `json:"chainId"`
	DeviceID string `json:"deviceId"`
}

type ExportedEmbeddedRequest struct {
	Address string `json:"address"`
}

type EmbeddedResponse struct {
	Share        string `json:"share"`
	Address      string `json:"address"`
	ChainID      int64  `json:"chainId"`
	DeviceID     string `json:"deviceId"`
	Device       string `json:"device"`
	Account      string `json:"account"`
	OwnerAddress string `json:"ownerAddress"`
	AccountType  string `json:"accountType"`
	Signer       string `json:"signer"`
}

type CreateDeviceRequest struct {
	AccountId string `json:"accountId"`
	Address   string `json:"address"`
	ChainId   int64  `json:"chainId"`
	Share     string `json:"share"`
}

type Device struct {
	gorm.Model
	ID        string `gorm:"primaryKey" json:"id"`
	Share     string `json:"share"`
	IsPrimary bool   `json:"isPrimary"`
	SignerId  string `json:"signerId"`
}

type Signer struct {
	gorm.Model
	ID string `gorm:"primaryKey" json:"id"`
}

type Account struct {
	gorm.Model
	ID           string `gorm:"primaryKey" json:"id"`
	Address      string `json:"address"`
	Username     string `json:"username"` // also referred as userId in the code
	ChainId      int64  `json:"chainId"`
	AuthProvider string `json:"auth_provider"`
	SignerId     string `json:"signerId"`
}

type DeviceResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"createdAt"`
	Address   string `json:"address"`
	Share     string `json:"share"`
	IsPrimary bool   `json:"isPrimary"`
}

type DeviceListResponse struct {
	Object string           `json:"object"`
	URL    string           `json:"url"`
	Data   []DeviceResponse `json:"data"`
	Start  int              `json:"start"`
	End    int              `json:"end"`
	Total  int              `json:"total"`
}

type AccountResponse struct {
	ID       string `gorm:"primaryKey" json:"id"`
	Address  string `json:"address"`
	Username string `json:"username"`
	ChainId  int64  `json:"chainId"`
	SignerId string `json:"signerId"`
}

type AccountListResponse struct {
	Object string            `json:"object"`
	URL    string            `json:"url"`
	Data   []AccountResponse `json:"data"`
	Start  int               `json:"start"`
	End    int               `json:"end"`
	Total  int               `json:"total"`
}

type CreateDeviceResponse = DeviceResponse
type GetDeviceResponse = DeviceResponse

type CreateEmbeddedRequestV2 struct {
	AccountType string  `json:"accountType"`
	ChainType   string  `json:"chainType"`
	ChainId     int64   `json:"chainId"`
	Address     string  `json:"address"`
	Share       string  `json:"share"`
	SignerUuid  *string `json:"signerUuid"`
}

type RecoverEmbeddedRequestV2 struct {
	Account string `json:"account"`
}

type SwitchChainQueriesV2 struct {
	Account string `json:"account"`
	ChainId int64  `json:"chainId"`
}

type RegisterRequestV2 struct {
	Account string `json:"account"`
	Share   string `json:"share"`
}

type GetSignerResponse struct {
	Id string `json:"id"`
}

type SwitchChainResponseV2 struct {
	Id           string `json:"id"` // account ID
	User         string `json:"user"`
	AccountType  string `json:"accountType"` // EOA, Smart Account etc.
	Address      string `json:"address"`
	OwnerAddress string `json:"ownerAddress"`
	ChainType    string `json:"chainType"`
	ChainId      int64  `json:"chainId"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type RecoverResponseV2 struct {
	Id            string `json:"id"`
	Account       string `json:"account"`
	SignerAddress string `json:"signerAddress"`
	Signer        string `json:"signer"`
	Share         string `json:"share"`
	IsPrimary     bool   `json:"isPrimary"`
	User          string `json:"user"`
}
