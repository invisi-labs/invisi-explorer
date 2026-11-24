package model

// TransactionObservation contains canonical outer-transaction facts only. It
// does not establish internal call paths, delegation or beneficial ownership.
type TransactionObservation struct {
	ChainID           string `json:"chainId"`
	ProtocolAddress   string `json:"protocolAddress"`
	TxHash            string `json:"txHash"`
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	TransactionType   string `json:"transactionType"`
	TransactionTo     string `json:"transactionTo,omitempty"`
	TransactionSender string `json:"transactionSender,omitempty"`
}
