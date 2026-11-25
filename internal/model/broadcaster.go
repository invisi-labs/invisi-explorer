package model

import "github.com/invisi-labs/invisi-explorer/internal/publicchain"

type BroadcasterStatus struct {
	APIVersion       string `json:"apiVersion"`
	ProtocolVersion  string `json:"protocolVersion"`
	ChainID          string `json:"chainId"`
	Protocol         string `json:"protocol"`
	RuntimeCodeHash  string `json:"runtimeCodeHash"`
	Sender           string `json:"sender"`
	FeeRecipient     string `json:"feeRecipient"`
	SubmissionHalted *bool  `json:"submissionHalted"`
	Scheduling       *struct {
		Enabled                       *bool  `json:"enabled"`
		MaxDelaySeconds               string `json:"maxDelaySeconds"`
		DefaultExecutionWindowSeconds string `json:"defaultExecutionWindowSeconds"`
		MaxExecutionWindowSeconds     string `json:"maxExecutionWindowSeconds"`
	} `json:"scheduling,omitempty"`
	Provenance *struct {
		Enabled            *bool  `json:"enabled"`
		Health             string `json:"health"`
		CheckedAt          string `json:"checkedAt,omitempty"`
		AggregateSupported *bool  `json:"aggregateSupported"`
		TrackingSupported  *bool  `json:"trackingSupported"`
	} `json:"provenance,omitempty"`
	PaymentRouters *[]struct {
		Router                string `json:"router"`
		Core                  string `json:"core"`
		RuntimeCodeHash       string `json:"runtimeCodeHash"`
		AccountImplementation string `json:"accountImplementation,omitempty"`
	} `json:"paymentRouters,omitempty"`
	FeeLock *struct {
		Core   *bool `json:"core"`
		Router *bool `json:"router"`
	} `json:"feeLock,omitempty"`
}
type BroadcasterToken struct {
	TokenAddress    string `json:"tokenAddress"`
	Supported       bool   `json:"supported"`
	Status          string `json:"status"`
	PriceValidUntil string `json:"priceValidUntil,omitempty"`
}
type BroadcasterObservation struct {
	ExpectedCodeHash string             `json:"expectedCodeHash,omitempty"`
	Stale            bool               `json:"stale"`
	Status           string             `json:"status"`
	CheckedAt        int64              `json:"checkedAt"`
	ResponseMillis   int64              `json:"responseMillis"`
	IdentityStatus   string             `json:"identityStatus"`
	Service          *BroadcasterStatus `json:"service,omitempty"`
	Tokens           []BroadcasterToken `json:"tokens"`
	TokensStatus     string             `json:"tokensStatus"`
	TokensCheckedAt  int64              `json:"tokensCheckedAt,omitempty"`
}
type BroadcasterEntry struct {
	publicchain.Broadcaster
	ChainID         string                 `json:"chainId"`
	ProtocolAddress string                 `json:"protocolAddress"`
	ID              string                 `json:"id"`
	Observation     BroadcasterObservation `json:"observation"`
}
type BroadcasterDirectory struct {
	ChainID          string             `json:"chainId"`
	ProtocolAddress  string             `json:"protocolAddress"`
	RegistryAddress  string             `json:"registryAddress"`
	RegistryCodeHash string             `json:"registryCodeHash"`
	Anchor           publicchain.Anchor `json:"anchor"`
	Status           string             `json:"status"`
	CheckedAt        int64              `json:"checkedAt"`
	Complete         bool               `json:"complete"`
	Items            []BroadcasterEntry `json:"items"`
}
