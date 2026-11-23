// Package publicchain defines public state returned through the configured Protocol RPC.
package publicchain

type Scope struct {
	ChainID         string `json:"chainId"`
	ProtocolAddress string `json:"protocolAddress"`
}
type Anchor struct {
	BlockNumber string `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
	ParentHash  string `json:"parentHash,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}
type TreeState struct {
	Scope
	Anchor           Anchor `json:"anchor"`
	Namespace        string `json:"namespace"`
	Status           string `json:"status"`
	TreeID           string `json:"treeId"`
	TreeCount        string `json:"treeCount,omitempty"`
	ActiveTreeID     string `json:"activeTreeId,omitempty"`
	Depth            string `json:"depth"`
	Capacity         string `json:"capacity"`
	NextLeafIndex    string `json:"nextLeafIndex"`
	LeafCount        string `json:"leafCount"`
	Root             string `json:"root"`
	IsFull           bool   `json:"isFull"`
	IsActive         *bool  `json:"isActive,omitempty"`
	RootHistoryKnown *bool  `json:"rootHistoryKnown,omitempty"`
}
type GrantState struct {
	Scope
	Anchor       Anchor `json:"anchor"`
	Owner        string `json:"owner"`
	TokenAddress string `json:"tokenAddress"`
	GrantID      string `json:"grantId"`
	Status       string `json:"status"`
}
type DirectoryQuery struct {
	Anchor Anchor
	After  string
	Limit  int
}
type Broadcaster struct {
	Operator      string `json:"operator"`
	Endpoint      string `json:"endpoint"`
	MetadataURI   string `json:"metadataURI"`
	QuoteSigner   string `json:"quoteSigner"`
	FeeRecipient  string `json:"feeRecipient"`
	Enabled       bool   `json:"enabled"`
	UpdatedAt     Anchor `json:"updatedAt"`
	ServiceHealth string `json:"serviceHealth"`
}
type DirectoryPage struct {
	Scope
	Anchor           Anchor        `json:"anchor"`
	RegistryAddress  string        `json:"registryAddress,omitempty"`
	RegistryCodeHash string        `json:"registryCodeHash,omitempty"`
	FromBlock        string        `json:"fromBlock,omitempty"`
	Status           string        `json:"status"`
	Reason           string        `json:"reason,omitempty"`
	Items            []Broadcaster `json:"items"`
	NextOperator     string        `json:"nextOperator,omitempty"`
}
type Error struct{ Code string }

func (e *Error) Error() string  { return e.Code }
func failure(code string) error { return &Error{Code: code} }
