package model

type AssetTransfer struct {
	EventKind    string `json:"eventKind,omitempty"`
	Kind         string `json:"kind"`
	TokenAddress string `json:"tokenAddress"`
	From         string `json:"from"`
	To           string `json:"to"`
	Amount       string `json:"amount"`
	CallPath     string `json:"callPath"`
	ContextPath  string `json:"contextPath"`
	LogIndex     string `json:"logIndex,omitempty"`
	Order        string `json:"order"`
}

// FundingEvidence binds public Core funding to authenticated transaction context.
type FundingEvidence struct {
	Dependencies      []string        `json:"dependencies,omitempty"`
	Version           string          `json:"version"`
	TransactionSender string          `json:"transactionSender"`
	CoreCallPath      string          `json:"coreCallPath"`
	Coverage          string          `json:"coverage"`
	EvidenceDigest    string          `json:"evidenceDigest"`
	Transfers         []AssetTransfer `json:"transfers"`
}

// FundingEvidenceDigest hashes canonical evidence independently of its own digest.
// Addresses are case-insensitive; transfer order is the authenticated trace order.
