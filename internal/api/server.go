package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"settletally/internal/reconcile"
)

const (
	defaultLookback = uint64(500)
	defaultMaxSpan  = uint64(5_000)
	defaultMaxRows  = 2_000
	maxRequestBytes = 2 << 20
)

type ChainReader interface {
	LatestBlock(context.Context) (uint64, error)
	ScanWallet(context.Context, string, uint64, uint64) ([]reconcile.Payment, error)
}

type Config struct {
	AllowedOrigins []string
	Lookback       uint64
	MaxBlockSpan   uint64
	MaxRecords     int
	RequestTimeout time.Duration
	MaxConcurrent  int
	Logger         *slog.Logger
}

type Server struct {
	chain          ChainReader
	allowedOrigins map[string]struct{}
	lookback       uint64
	maxBlockSpan   uint64
	maxRecords     int
	requestTimeout time.Duration
	semaphore      chan struct{}
	logger         *slog.Logger
}

type expectedInput struct {
	Reference    string              `json:"reference"`
	Direction    reconcile.Direction `json:"direction"`
	Amount       string              `json:"amount"`
	Counterparty string              `json:"counterparty"`
	DueDate      string              `json:"dueDate,omitempty"`
	MemoID       string              `json:"memoId,omitempty"`
}

type reconcileRequest struct {
	Wallet          string          `json:"wallet"`
	FromBlock       *uint64         `json:"fromBlock,omitempty"`
	ToBlock         *uint64         `json:"toBlock,omitempty"`
	ExpectedRecords []expectedInput `json:"expectedRecords"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewServer(chain ChainReader, config Config) *Server {
	if config.Lookback == 0 {
		config.Lookback = defaultLookback
	}
	if config.MaxBlockSpan == 0 {
		config.MaxBlockSpan = defaultMaxSpan
	}
	if config.MaxRecords == 0 {
		config.MaxRecords = defaultMaxRows
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 45 * time.Second
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 4
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	allowed := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return &Server{
		chain:          chain,
		allowedOrigins: allowed,
		lookback:       config.Lookback,
		maxBlockSpan:   config.MaxBlockSpan,
		maxRecords:     config.MaxRecords,
		requestTimeout: config.RequestTimeout,
		semaphore:      make(chan struct{}, config.MaxConcurrent),
		logger:         config.Logger,
	}
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("POST /api/v1/reconcile", server.reconcile)
	mux.HandleFunc("OPTIONS /api/v1/reconcile", server.options)
	return server.withSecurityHeaders(server.withCORS(mux))
}

func (server *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{
		"status":  "ok",
		"network": "Arc Testnet",
	})
}

func (server *Server) options(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusNoContent)
}

func (server *Server) reconcile(writer http.ResponseWriter, request *http.Request) {
	select {
	case server.semaphore <- struct{}{}:
		defer func() { <-server.semaphore }()
	default:
		writeError(writer, http.StatusTooManyRequests, "the service is busy; retry shortly")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input reconcileRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid request body: "+friendlyJSONError(err))
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(writer, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	expected, err := server.validateInput(input)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	input.Wallet = reconcile.NormalizeAddress(input.Wallet)

	ctx, cancel := context.WithTimeout(request.Context(), server.requestTimeout)
	defer cancel()
	latest, err := server.chain.LatestBlock(ctx)
	if err != nil {
		server.logger.Error("read Arc latest block", "error", err)
		writeError(writer, http.StatusBadGateway, "Arc RPC is temporarily unavailable")
		return
	}
	fromBlock, toBlock, err := server.resolveRange(input, latest)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	payments, err := server.chain.ScanWallet(ctx, input.Wallet, fromBlock, toBlock)
	if err != nil {
		server.logger.Error("scan Arc wallet", "error", err, "fromBlock", fromBlock, "toBlock", toBlock)
		writeError(writer, http.StatusBadGateway, "Arc activity could not be read; retry shortly")
		return
	}
	report := reconcile.BuildReport(input.Wallet, fromBlock, toBlock, expected, payments)
	writeJSON(writer, http.StatusOK, report)
}

func (server *Server) validateInput(input reconcileRequest) ([]reconcile.ExpectedRecord, error) {
	input.Wallet = reconcile.NormalizeAddress(input.Wallet)
	if !reconcile.ValidAddress(input.Wallet) {
		return nil, fmt.Errorf("wallet must be a valid EVM address")
	}
	if len(input.ExpectedRecords) == 0 {
		return nil, fmt.Errorf("expectedRecords must contain at least one record")
	}
	if len(input.ExpectedRecords) > server.maxRecords {
		return nil, fmt.Errorf("expectedRecords exceeds the %d-record limit", server.maxRecords)
	}

	records := make([]reconcile.ExpectedRecord, 0, len(input.ExpectedRecords))
	seen := make(map[string]int, len(input.ExpectedRecords))
	for index, value := range input.ExpectedRecords {
		row := index + 1
		reference := strings.TrimSpace(value.Reference)
		if reference == "" {
			return nil, fmt.Errorf("expectedRecords[%d].reference is required", index)
		}
		key := strings.ToLower(reference)
		if previous, exists := seen[key]; exists {
			return nil, fmt.Errorf("expectedRecords[%d].reference duplicates record %d", index, previous)
		}
		seen[key] = row
		if value.Direction != reconcile.DirectionInbound && value.Direction != reconcile.DirectionOutbound && value.Direction != reconcile.DirectionSelf {
			return nil, fmt.Errorf("expectedRecords[%d].direction must be inbound, outbound, or self", index)
		}
		amount, err := reconcile.ParseUSDC(value.Amount)
		if err != nil {
			return nil, fmt.Errorf("expectedRecords[%d].amount: %w", index, err)
		}
		counterparty := reconcile.NormalizeAddress(value.Counterparty)
		if !reconcile.ValidAddress(counterparty) {
			return nil, fmt.Errorf("expectedRecords[%d].counterparty must be a valid EVM address", index)
		}
		var dueDate *time.Time
		if raw := strings.TrimSpace(value.DueDate); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return nil, fmt.Errorf("expectedRecords[%d].dueDate must use YYYY-MM-DD", index)
			}
			dueDate = &parsed
		}
		records = append(records, reconcile.ExpectedRecord{
			Reference:    reference,
			Direction:    value.Direction,
			AmountMicros: amount,
			Counterparty: counterparty,
			DueDate:      dueDate,
			MemoID:       strings.ToLower(strings.TrimSpace(value.MemoID)),
		})
	}
	return records, nil
}

func (server *Server) resolveRange(input reconcileRequest, latest uint64) (uint64, uint64, error) {
	toBlock := latest
	if input.ToBlock != nil {
		toBlock = *input.ToBlock
		if toBlock > latest {
			return 0, 0, fmt.Errorf("toBlock exceeds the current Arc block %d", latest)
		}
	}
	fromBlock := uint64(0)
	if toBlock > server.lookback {
		fromBlock = toBlock - server.lookback
	}
	if input.FromBlock != nil {
		fromBlock = *input.FromBlock
	}
	if fromBlock > toBlock {
		return 0, 0, fmt.Errorf("fromBlock must not exceed toBlock")
	}
	if toBlock-fromBlock > server.maxBlockSpan {
		return 0, 0, fmt.Errorf("block range exceeds the %d-block limit", server.maxBlockSpan)
	}
	return fromBlock, toBlock, nil
}

func (server *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if origin != "" {
			_, configured := server.allowedOrigins[origin]
			if !configured && !sameOrigin(origin, request.Host) {
				writeError(writer, http.StatusForbidden, "origin is not allowed")
				return
			}
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.Header().Set("Vary", "Origin")
		}
		next.ServeHTTP(writer, request)
	})
}

func sameOrigin(origin, host string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Host, host)
}

func (server *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(writer, request)
	})
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, errorResponse{Error: message})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func friendlyJSONError(err error) string {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return "body exceeds " + strconv.FormatInt(maxRequestBytes, 10) + " bytes"
	}
	if errors.Is(err, context.Canceled) {
		return "request was canceled"
	}
	return err.Error()
}
