package reconcile

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDecodeLiveArcMemoFixture(t *testing.T) {
	t.Parallel()
	logs := []rpcLog{{
		Address: ArcMemoAddress,
		Topics: []string{
			memoTopic,
			"0x0000000000000000000000009a605c93932f729d0bee62899d1ccadc11a9b4bc",
			"0x0000000000000000000000003600000000000000000000000000000000000000",
			"0x922b866a8435fae2e426499b327ce0119ac3a81fb6e38cf4ad32caf5c9a032b4",
		},
		Data:     "0x88a1319533b7c39c7a07a4c464340d8d0d265d401f3de274ce3ea24b6aea23580000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000004fb5d000000000000000000000000000000000000000000000000000000000000000a70697a7a612066756e6400000000000000000000000000000000000000000000",
		LogIndex: "0x5",
	}}
	memos, err := extractMemos(logs)
	if err != nil {
		t.Fatal(err)
	}
	if len(memos) != 1 || memos[0].Text != "pizza fund" {
		t.Fatalf("unexpected memos: %#v", memos)
	}
}

func TestTransferPaymentUsesCanonicalSixDecimalAmount(t *testing.T) {
	t.Parallel()
	wallet := "0x9a605c93932f729d0bee62899d1ccadc11a9b4bc"
	log := rpcLog{
		Address: ArcUSDCAddress,
		Topics: []string{
			transferTopic,
			"0x0000000000000000000000009a605c93932f729d0bee62899d1ccadc11a9b4bc",
			"0x0000000000000000000000009a605c93932f729d0bee62899d1ccadc11a9b4bc",
		},
		Data:            "0x000000000000000000000000000000000000000000000000000000000000841e",
		BlockNumber:     "0x343febd",
		TransactionHash: "0xaaaa",
		LogIndex:        "0x4",
	}
	payment, err := transferPayment(log, wallet)
	if err != nil {
		t.Fatal(err)
	}
	if payment.AmountMicros != 33_822 || payment.Direction != DirectionSelf {
		t.Fatalf("unexpected payment: %#v", payment)
	}
}

func TestReceiptFeeMicros(t *testing.T) {
	t.Parallel()
	fee, err := receiptFeeMicros(rpcReceipt{GasUsed: "0xef88", EffectiveGasPrice: "0x5d21dba00"})
	if err != nil {
		t.Fatal(err)
	}
	if fee != 1_533 {
		t.Fatalf("fee = %d micros, want 1533", fee)
	}
}

func TestRPCClientFailsOverAfterRateLimit(t *testing.T) {
	t.Parallel()
	client := NewRPCClient("https://limited.test,https://healthy.test")
	client.HTTPClient.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "limited.test" {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"error":{"code":-32011,"message":"request limit reached"}}`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":"0x2a"}`)),
			Header:     make(http.Header),
		}, nil
	})
	block, err := client.LatestBlock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if block != 42 {
		t.Fatalf("block = %d, want 42", block)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
