package model

import (
	"strings"

	p "github.com/invisi-labs/invisi-explorer/internal/wire"
)

type Anchor struct {
	BlockNumber string `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
}
type Funding struct {
	Payer        string `json:"payer"`
	TokenAddress string `json:"tokenAddress"`
	Amount       string `json:"amount"`
}
type GrantRef struct {
	PayerIndex string `json:"payerIndex"`
	GrantID    string `json:"grantId"`
	Payer      string `json:"payer,omitempty"`
}
type CreditOutput struct {
	CreditCommitment  p.Field `json:"creditCommitment"`
	TreeID            string  `json:"treeId"`
	LeafIndex         string  `json:"leafIndex"`
	OwnerCiphertext   string  `json:"ownerCiphertext,omitempty"`
	SpenderCiphertext string  `json:"spenderCiphertext,omitempty"`
	OwnerClue         string  `json:"ownerClue,omitempty"`
	SpenderClue       string  `json:"spenderClue,omitempty"`
}
type NoteOutput struct {
	NoteCommitment p.Field `json:"noteCommitment"`
	TreeID         string  `json:"treeId"`
	LeafIndex      string  `json:"leafIndex"`
	Amount         string  `json:"amount,omitempty"`
	Ciphertext     string  `json:"ciphertext,omitempty"`
	Clue           string  `json:"clue,omitempty"`
}
type Operation struct {
	CreditSlotIndexed   bool              `json:"creditSlotIndexed,omitempty"`
	ExecutionContext    *ExecutionContext `json:"executionContext,omitempty"`
	ExecutionPath       string            `json:"executionPath,omitempty"`
	FundingEvidence     *FundingEvidence  `json:"fundingEvidence,omitempty"`
	RouterFunding       *RouterFunding    `json:"routerFunding,omitempty"`
	ChainID             string            `json:"chainId"`
	ProtocolAddress     string            `json:"protocolAddress"`
	BlockNumber         string            `json:"blockNumber"`
	BlockHash           string            `json:"blockHash"`
	ParentHash          string            `json:"parentHash"`
	Timestamp           string            `json:"timestamp"`
	TxHash              string            `json:"txHash"`
	OperationIndex      string            `json:"operationIndex"`
	MemberIndex         string            `json:"memberIndex,omitempty"`
	LogIndex            string            `json:"logIndex"`
	Kind                string            `json:"kind"`
	TokenAddress        string            `json:"tokenAddress"`
	MainCircuitCapacity string            `json:"mainCircuitCapacity"`
	PublicInputs        []p.Field         `json:"publicInputs"`
	NoteNullifiers      []p.Field         `json:"noteNullifiers"`
	CreditNullifiers    []p.Field         `json:"creditNullifiers"`
	NoteOutputs         []NoteOutput      `json:"noteOutputs"`
	CreditOutputs       []CreditOutput    `json:"creditOutputs"`
	GrantRefs           []GrantRef        `json:"grantRefs"`
	Funding             []Funding         `json:"funding"`
	WithdrawalAmount    string            `json:"withdrawalAmount"`
	FeeAmount           string            `json:"feeAmount"`
	FeeRecipient        string            `json:"feeRecipient"`
	Success             bool              `json:"success"`
	OperationDigest     string            `json:"operationDigest"`
	UnsupportedReason   string            `json:"unsupportedReason,omitempty"`
	GasUsed             string            `json:"gasUsed,omitempty"`
	EffectiveGasPrice   string            `json:"effectiveGasPrice,omitempty"`
	Canonical           bool              `json:"canonical"`
}

func (o Operation) ID() string {
	return o.ChainID + "/" + strings.ToLower(o.ProtocolAddress) + "/" + o.BlockHash + "/" + o.TxHash + "/" + o.OperationIndex
}
func (o Operation) Origin() string {
	return o.ChainID + "/" + strings.ToLower(o.ProtocolAddress) + "/" + o.BlockHash + "/" + o.TxHash + "/" + o.OperationIndex + "/" + o.OperationDigest
}
func (o Operation) Anchor() Anchor { return Anchor{o.BlockNumber, o.BlockHash} }

type OperationsPage struct {
	Operations  []Operation `json:"operations"`
	Snapshot    Anchor      `json:"snapshot"`
	IndexedHead Anchor      `json:"indexedHead"`
	NextCursor  string      `json:"nextCursor,omitempty"`
}
type Recognition struct {
	Scope              p.Scope `json:"scope"`
	OperationID        string  `json:"operationId"`
	State              string  `json:"recognitionState"`
	ReasonCode         string  `json:"reasonCode,omitempty"`
	Generation         string  `json:"chainGeneration"`
	CheckpointSequence string  `json:"checkpointSequence,omitempty"`
	ProofJobID         string  `json:"proofJobId,omitempty"`
	FirstRecognizedAt  string  `json:"firstRecognizedAt,omitempty"`
}
type NoteRecognition struct {
	Scope              p.Scope `json:"scope"`
	NoteCommitment     p.Field `json:"noteCommitment"`
	Leaf               p.Field `json:"leaf"`
	Generation         string  `json:"chainGeneration"`
	TreeID             string  `json:"treeId"`
	LeafIndex          string  `json:"leafIndex"`
	OperationID        string  `json:"operationId"`
	CheckpointSequence string  `json:"checkpointSequence"`
	RecognizedAt       string  `json:"recognizedAt"`
}
