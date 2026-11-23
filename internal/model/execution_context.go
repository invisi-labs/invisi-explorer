package model

type ExecutionContext struct {
	TransactionType   string                   `json:"transactionType"`
	TransactionSender string                   `json:"transactionSender"`
	TransactionTo     string                   `json:"transactionTo,omitempty"`
	CoreCallPath      string                   `json:"coreCallPath"`
	Calls             []ExecutionCall          `json:"calls"`
	Authorizations    []ExecutionAuthorization `json:"authorizations,omitempty"`
}
type ExecutionCall struct {
	Path string `json:"path"`
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ExecutionAuthorization records signed intent only. SignatureValid means that
// the authorization signature recovers Authority; it does not imply applicability,
// successful installation, or effective delegation during this transaction.
type ExecutionAuthorization struct {
	ChainID        string `json:"chainId"`
	Address        string `json:"address"`
	Authority      string `json:"authority,omitempty"`
	Nonce          string `json:"nonce"`
	SignatureValid bool   `json:"signatureValid"`
}
