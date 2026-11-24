package explorer

import (
	"encoding/hex"
	"github.com/invisi-labs/invisi-explorer/internal/model"
	p "github.com/invisi-labs/invisi-explorer/internal/wire"
	"math/big"
	"strconv"
	"strings"
)

// Canonical TransactStatement layout: 15+3*capacity public fields; withdrawal
// address/amount follow the two delivery-hash limbs at offset 6+3*capacity.
// This exposes public statement data, never a private Note recipient.
func withdrawalAddress(op model.Operation) string {
	if !op.Canonical || !op.Success || op.Kind != "transact" || op.UnsupportedReason != "" {
		return ""
	}
	capacity, err := strconv.Atoi(op.MainCircuitCapacity)
	if err != nil || capacity < 1 || capacity > 190 || len(op.PublicInputs) != 15+3*capacity {
		return ""
	}
	values := make([]*big.Int, len(op.PublicInputs))
	for i, f := range op.PublicInputs {
		bytes, err := p.DecodeHex(string(f), 32)
		if err != nil {
			return ""
		}
		values[i] = new(big.Int).SetBytes(bytes)
	}
	chain, ok := new(big.Int).SetString(op.ChainID, 10)
	if !ok || values[0].Cmp(chain) != 0 {
		return ""
	}
	for i, address := range map[int]string{1: op.ProtocolAddress, 2: op.TokenAddress} {
		raw, err := p.DecodeHex(address, 20)
		if err != nil || values[i].Cmp(new(big.Int).SetBytes(raw)) != 0 {
			return ""
		}
	}
	offset := 6 + 3*capacity
	amount, ok := new(big.Int).SetString(op.WithdrawalAmount, 10)
	if !ok || amount.Sign() <= 0 || amount.BitLen() > 128 || values[offset+3].Cmp(amount) != 0 {
		return ""
	}
	recipient := values[offset+2]
	if recipient.Sign() == 0 || recipient.BitLen() > 160 {
		return ""
	}
	return "0x" + strings.ToLower(hex.EncodeToString(recipient.FillBytes(make([]byte, 20))))
}
