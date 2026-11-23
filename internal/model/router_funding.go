package model

type RouterFunding struct {
	RouterAddress   string `json:"routerAddress"`
	PlayerAddress   string `json:"playerAddress"`
	Channel         string `json:"channel"`
	NativeBudget    string `json:"nativeBudget"`
	NativeSpent     string `json:"nativeSpent"`
	NativeRefund    string `json:"nativeRefund"`
	RefundRecipient string `json:"refundRecipient"`
	ProofHash       string `json:"proofHash"`
	PaymentLogIndex string `json:"paymentLogIndex"`
}

// ValidateRouterFunding checks internal consistency. The ingest validator must
// independently authenticate the direct signed Router call and its receipt.
