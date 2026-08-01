package reconcile

import "time"

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
	DirectionSelf     Direction = "self"
)

type MatchStatus string

const (
	StatusMatched     MatchStatus = "matched"
	StatusPartial     MatchStatus = "partial"
	StatusOverpaid    MatchStatus = "overpaid"
	StatusNeedsReview MatchStatus = "needs_review"
	StatusUnmatched   MatchStatus = "unmatched"
	StatusMissing     MatchStatus = "missing_payment"
	StatusAwaiting    MatchStatus = "awaiting_payment"
)

type ExpectedRecord struct {
	Reference    string     `json:"reference"`
	Direction    Direction  `json:"direction"`
	AmountMicros int64      `json:"amountMicros"`
	Counterparty string     `json:"counterparty"`
	DueDate      *time.Time `json:"dueDate,omitempty"`
	MemoID       string     `json:"memoId,omitempty"`
}

type Payment struct {
	TransactionHash string    `json:"transactionHash"`
	BlockNumber     uint64    `json:"blockNumber"`
	LogIndex        uint64    `json:"logIndex"`
	From            string    `json:"from"`
	To              string    `json:"to"`
	Direction       Direction `json:"direction"`
	AmountMicros    int64     `json:"amountMicros"`
	FeeMicros       int64     `json:"feeMicros"`
	MemoID          string    `json:"memoId,omitempty"`
	MemoText        string    `json:"memoText,omitempty"`
}

type Match struct {
	Expected         *ExpectedRecord `json:"expected"`
	Payments         []Payment       `json:"payments"`
	Status           MatchStatus     `json:"status"`
	MatchedMicros    int64           `json:"matchedMicros"`
	DifferenceMicros int64           `json:"differenceMicros"`
	Reason           string          `json:"reason"`
	Confidence       string          `json:"confidence"`
}

type Report struct {
	GeneratedAt       time.Time `json:"generatedAt"`
	Wallet            string    `json:"wallet"`
	FromBlock         uint64    `json:"fromBlock"`
	ToBlock           uint64    `json:"toBlock"`
	ExpectedCount     int       `json:"expectedCount"`
	PaymentCount      int       `json:"paymentCount"`
	ExpectedMicros    int64     `json:"expectedMicros"`
	MatchedMicros     int64     `json:"matchedMicros"`
	ReviewMicros      int64     `json:"reviewMicros"`
	UnmatchedMicros   int64     `json:"unmatchedMicros"`
	FeeMicros         int64     `json:"feeMicros"`
	Matches           []Match   `json:"matches"`
	UnmatchedPayments []Payment `json:"unmatchedPayments"`
}
