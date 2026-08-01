package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"settletally/reconcile"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		wallet       = flag.String("wallet", "", "Arc wallet address to reconcile")
		expectedPath = flag.String("expected", "", "CSV file containing expected records")
		rpcURL       = flag.String("rpc", "https://rpc.testnet.arc.io,https://rpc.testnet.arc.network", "comma-separated Arc JSON-RPC URLs used in failover order")
		fromBlock    = flag.Uint64("from-block", 0, "first block to scan; defaults to latest minus 500")
		toBlock      = flag.Uint64("to-block", 0, "last block to scan; defaults to latest")
		outDir       = flag.String("out", "out", "directory for JSON and CSV reports")
		batchSize    = flag.Uint64("batch-size", 250, "maximum block range per eth_getLogs request")
	)
	flag.Parse()
	if !reconcile.ValidAddress(*wallet) {
		return fmt.Errorf("--wallet must be a valid EVM address")
	}
	if *expectedPath == "" {
		return fmt.Errorf("--expected is required")
	}

	expectedFile, err := os.Open(*expectedPath)
	if err != nil {
		return fmt.Errorf("open expected-record CSV: %w", err)
	}
	expected, err := reconcile.ReadExpectedCSV(expectedFile)
	closeErr := expectedFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	client := reconcile.NewRPCClient(*rpcURL)
	client.BatchSize = *batchSize
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	latest, err := client.LatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("read latest Arc block: %w", err)
	}
	end := *toBlock
	if end == 0 || end > latest {
		end = latest
	}
	start := *fromBlock
	if start == 0 {
		if end > 500 {
			start = end - 500
		}
	}
	if start > end {
		return fmt.Errorf("scan start block %d exceeds end block %d", start, end)
	}

	fmt.Printf("Scanning Arc blocks %d to %d for %s\n", start, end, reconcile.NormalizeAddress(*wallet))
	payments, err := client.ScanWallet(ctx, *wallet, start, end)
	if err != nil {
		return err
	}
	report := reconcile.BuildReport(*wallet, start, end, expected, payments)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	jsonPath := filepath.Join(*outDir, "reconciliation.json")
	csvPath := filepath.Join(*outDir, "reconciliation.csv")
	if err := writeFile(jsonPath, func(file *os.File) error { return reconcile.WriteReportJSON(file, report) }); err != nil {
		return err
	}
	if err := writeFile(csvPath, func(file *os.File) error { return reconcile.WriteReportCSV(file, report) }); err != nil {
		return err
	}

	fmt.Printf("Expected records: %d\n", report.ExpectedCount)
	fmt.Printf("Onchain payments: %d\n", report.PaymentCount)
	fmt.Printf("Matched amount: %s USDC\n", reconcile.FormatUSDC(report.MatchedMicros))
	fmt.Printf("Unmatched amount: %s USDC\n", reconcile.FormatUSDC(report.UnmatchedMicros))
	fmt.Printf("Wallet fees: %s USDC\n", reconcile.FormatUSDC(report.FeeMicros))
	fmt.Printf("JSON report: %s\n", jsonPath)
	fmt.Printf("CSV report: %s\n", csvPath)
	return nil
}

func writeFile(path string, write func(*os.File) error) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if err := write(file); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
