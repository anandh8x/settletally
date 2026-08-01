package service

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	api "settletally/httpapi"
	"settletally/reconcile"
)

func NewHandler(logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	chain := reconcile.NewRPCClient(envOr("ARC_RPC_URLS", "https://rpc.testnet.arc.io,https://rpc.testnet.arc.network"))
	chain.BatchSize = uint64(envInt("ARC_RPC_BATCH_SIZE", 250))
	return api.NewServer(chain, api.Config{
		AllowedOrigins: strings.Split(envOr("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174"), ","),
		MaxBlockSpan:   uint64(envInt("MAX_BLOCK_SPAN", 5_000)),
		MaxRecords:     envInt("MAX_EXPECTED_RECORDS", 2_000),
		MaxConcurrent:  envInt("MAX_CONCURRENT_SCANS", 4),
		RequestTimeout: time.Duration(envInt("REQUEST_TIMEOUT_SECONDS", 45)) * time.Second,
		Logger:         logger,
	}).Handler()
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
