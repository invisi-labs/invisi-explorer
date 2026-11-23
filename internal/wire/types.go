package wire

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type Field string

const ZeroHash = "0x0000000000000000000000000000000000000000000000000000000000000000"
const Zero Field = Field(ZeroHash)

func ParseField(s string) (Field, error) {
	b, err := DecodeHex(s, 32)
	if err != nil {
		return "", err
	}
	modulus, _ := new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	if new(big.Int).SetBytes(b).Cmp(modulus) >= 0 {
		return "", errors.New("noncanonical field")
	}
	return Field("0x" + hex.EncodeToString(b)), nil
}
func (f *Field) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseField(s)
	if err == nil {
		*f = v
	}
	return err
}
func DecodeHex(s string, n int) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") || len(s) != 2+n*2 {
		return nil, fmt.Errorf("expected %d-byte hex", n)
	}
	b, e := hex.DecodeString(s[2:])
	return b, e
}
func Uint(s string, bits int) (*big.Int, error) {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 || v.BitLen() > bits || v.String() != s {
		return nil, errors.New("noncanonical unsigned integer")
	}
	return v, nil
}
func Uint64(s string) (uint64, error) {
	v, e := Uint(s, 64)
	if e != nil {
		return 0, e
	}
	return v.Uint64(), nil
}
func UintField(v uint64) Field { return Field(fmt.Sprintf("0x%064x", v)) }

type Scope struct {
	ChainID         string `json:"chainId"`
	ProtocolAddress string `json:"protocolAddress"`
}

func (s Scope) Validate() error {
	if _, e := Uint(s.ChainID, 64); e != nil {
		return e
	}
	if b, e := DecodeHex(s.ProtocolAddress, 20); e != nil || new(big.Int).SetBytes(b).Sign() == 0 {
		return errors.New("invalid scope")
	}
	return nil
}
func (s Scope) Key() string {
	return s.ChainID + "/" + strings.ToLower(s.ProtocolAddress)
}

type Certificate struct {
	CertificateVersion     string `json:"certificateVersion"`
	ProviderID             string `json:"providerId"`
	SigningKeyID           string `json:"signingKeyId"`
	ChainID                string `json:"chainId"`
	ProtocolAddress        string `json:"protocolAddress"`
	ChainGeneration        string `json:"chainGeneration"`
	RootSequence           string `json:"rootSequence"`
	TreeID                 string `json:"treeId"`
	LeafCount              string `json:"leafCount"`
	Root                   Field  `json:"root"`
	ConfirmedBlockNumber   string `json:"confirmedBlockNumber"`
	ConfirmedBlockHash     string `json:"confirmedBlockHash"`
	DecisionLogHash        string `json:"decisionLogHash"`
	PreviousCheckpointHash string `json:"previousCheckpointHash"`
	IssuedAt               string `json:"issuedAt"`
	Signature              string `json:"signature"`
}

func (c Certificate) Scope() Scope { return Scope{c.ChainID, c.ProtocolAddress} }
