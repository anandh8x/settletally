package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"settletally/internal/reconcile"
)

type fakeChain struct {
	latest      uint64
	payments    []reconcile.Payment
	latestError error
	scanError   error
	fromBlock   uint64
	toBlock     uint64
	wallet      string
}

func (chain *fakeChain) LatestBlock(context.Context) (uint64, error) {
	return chain.latest, chain.latestError
}

func (chain *fakeChain) ScanWallet(_ context.Context, wallet string, fromBlock, toBlock uint64) ([]reconcile.Payment, error) {
	chain.wallet = wallet
	chain.fromBlock = fromBlock
	chain.toBlock = toBlock
	return chain.payments, chain.scanError
}

func TestReconcileReturnsARealReportShape(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	counterparty := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	chain := &fakeChain{
		latest: 10_000,
		payments: []reconcile.Payment{{
			TransactionHash: "0x1234",
			BlockNumber:     9_900,
			From:            counterparty,
			To:              wallet,
			Direction:       reconcile.DirectionInbound,
			AmountMicros:    1_250_000,
			MemoText:        "INV-1042",
		}},
	}
	server := NewServer(chain, Config{Lookback: 500, RequestTimeout: time.Second}).Handler()
	body := `{
  "wallet":"0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  "expectedRecords":[{
    "reference":"INV-1042",
    "direction":"inbound",
    "amount":"1.25",
    "counterparty":"0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
  }]
}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reconcile", strings.NewReader(body))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if chain.fromBlock != 9_500 || chain.toBlock != 10_000 || chain.wallet != wallet {
		t.Fatalf("unexpected scan: wallet=%s blocks=%d-%d", chain.wallet, chain.fromBlock, chain.toBlock)
	}
	var report reconcile.Report
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Status != reconcile.StatusMatched {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.UnmatchedPayments == nil || report.Matches[0].Payments == nil {
		t.Fatalf("API arrays must encode as arrays, not null: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"generatedAt"`) || strings.Contains(response.Body.String(), `"GeneratedAt"`) {
		t.Fatalf("API did not use stable lower-camel JSON fields: %s", response.Body.String())
	}
}

func TestReconcileRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	t.Parallel()
	validRecord := `{"reference":"INV-1","direction":"inbound","amount":"1","counterparty":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "invalid wallet", body: `{"wallet":"nope","expectedRecords":[` + validRecord + `]}`, want: "wallet must be a valid"},
		{name: "unknown field", body: `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","extra":true,"expectedRecords":[` + validRecord + `]}`, want: "unknown field"},
		{name: "duplicate reference", body: `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expectedRecords":[` + validRecord + `,{"reference":"inv-1","direction":"inbound","amount":"2","counterparty":"0xcccccccccccccccccccccccccccccccccccccccc"}]}`, want: "duplicates record"},
		{name: "trailing object", body: `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expectedRecords":[` + validRecord + `]} {}`, want: "one JSON object"},
	}
	server := NewServer(&fakeChain{latest: 100}, Config{}).Handler()
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/reconcile", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReconcileEnforcesRangeAndOrigin(t *testing.T) {
	t.Parallel()
	chain := &fakeChain{latest: 10_000}
	server := NewServer(chain, Config{
		AllowedOrigins: []string{"https://settletally.example"},
		MaxBlockSpan:   100,
	}).Handler()
	body := `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","fromBlock":9000,"toBlock":10000,"expectedRecords":[{"reference":"INV-1","direction":"inbound","amount":"1","counterparty":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/reconcile", strings.NewReader(body))
	request.Header.Set("Origin", "https://malicious.example")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forbidden origin status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/reconcile", strings.NewReader(body))
	request.Header.Set("Origin", "https://settletally.example")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "block range exceeds") {
		t.Fatalf("range response = %d %s", response.Code, response.Body.String())
	}
}

func TestReconcileAllowsSameOriginWithoutConfiguration(t *testing.T) {
	t.Parallel()
	server := NewServer(&fakeChain{latest: 100}, Config{}).Handler()
	body := `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expectedRecords":[{"reference":"INV-1","direction":"inbound","amount":"1","counterparty":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`
	request := httptest.NewRequest(http.MethodPost, "https://settletally.vercel.app/api/v1/reconcile", strings.NewReader(body))
	request.Host = "settletally.vercel.app"
	request.Header.Set("Origin", "https://settletally.vercel.app")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("same-origin response = %d %s", response.Code, response.Body.String())
	}
}

func TestReconcileDoesNotLeakUpstreamErrors(t *testing.T) {
	t.Parallel()
	chain := &fakeChain{latestError: errors.New("provider secret and internal details")}
	server := NewServer(chain, Config{}).Handler()
	body := `{"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expectedRecords":[{"reference":"INV-1","direction":"inbound","amount":"1","counterparty":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reconcile", strings.NewReader(body))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || strings.Contains(response.Body.String(), "provider secret") {
		t.Fatalf("response leaked upstream details: %d %s", response.Code, response.Body.String())
	}
}

func TestHealthDoesNotCallArc(t *testing.T) {
	t.Parallel()
	server := NewServer(&fakeChain{latestError: errors.New("must not be called")}, Config{}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Arc Testnet") {
		t.Fatalf("health response = %d %s", response.Code, response.Body.String())
	}
}
