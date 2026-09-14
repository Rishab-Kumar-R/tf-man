package events

import "time"

type Type string

const (
	TypeBidPlaced      Type = "BidPlaced"
	TypeOrderPlaced    Type = "OrderPlaced"
	TypeOrderConfirmed Type = "OrderConfirmed"
	TypePaymentFailed  Type = "PaymentFailed"
	TypeFraudFlagged   Type = "FraudFlagged"
)

type Event struct {
	Type      Type           `json:"event_type"`
	UserID    string         `json:"user_id"`
	OrderID   string         `json:"order_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
