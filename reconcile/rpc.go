package reconcile

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ArcUSDCAddress = "0x3600000000000000000000000000000000000000"
	ArcMemoAddress = "0x5294e9927c3306dcbadb03fe70b92e01ccede505"
	transferTopic  = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	memoTopic      = "0xeb15ee720798341c37739df41be53acfbbf70ae6802dade35457beec6e47a5e4"
)

type RPCClient struct {
	URLs       []string
	HTTPClient *http.Client
	BatchSize  uint64
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type rpcLog struct {
	Address         string   `json:"address"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	BlockNumber     string   `json:"blockNumber"`
	TransactionHash string   `json:"transactionHash"`
	LogIndex        string   `json:"logIndex"`
}

type rpcReceipt struct {
	From              string   `json:"from"`
	Status            string   `json:"status"`
	GasUsed           string   `json:"gasUsed"`
	EffectiveGasPrice string   `json:"effectiveGasPrice"`
	Logs              []rpcLog `json:"logs"`
}

type memoData struct {
	ID       string
	Text     string
	LogIndex uint64
}

func NewRPCClient(urlList string) *RPCClient {
	var urls []string
	for _, value := range strings.Split(urlList, ",") {
		if value = strings.TrimSpace(value); value != "" {
			urls = append(urls, value)
		}
	}
	return &RPCClient{
		URLs:      urls,
		BatchSize: 250,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (client *RPCClient) LatestBlock(ctx context.Context) (uint64, error) {
	var result string
	if err := client.call(ctx, "eth_blockNumber", []any{}, &result); err != nil {
		return 0, err
	}
	return parseHexUint64(result)
}

func (client *RPCClient) ScanWallet(ctx context.Context, wallet string, fromBlock, toBlock uint64) ([]Payment, error) {
	wallet = NormalizeAddress(wallet)
	if !ValidAddress(wallet) {
		return nil, fmt.Errorf("wallet is not a valid EVM address")
	}
	if fromBlock > toBlock {
		return nil, fmt.Errorf("from block must not exceed to block")
	}
	batchSize := client.BatchSize
	if batchSize == 0 {
		batchSize = 250
	}
	paddedWallet := "0x" + strings.Repeat("0", 24) + wallet[2:]
	dedup := map[string]rpcLog{}

	for start := fromBlock; start <= toBlock; {
		end := start + batchSize - 1
		if end < start || end > toBlock {
			end = toBlock
		}
		filters := []map[string]any{
			{
				"address":   ArcUSDCAddress,
				"fromBlock": hexUint64(start),
				"toBlock":   hexUint64(end),
				"topics":    []any{transferTopic, paddedWallet},
			},
			{
				"address":   ArcUSDCAddress,
				"fromBlock": hexUint64(start),
				"toBlock":   hexUint64(end),
				"topics":    []any{transferTopic, nil, paddedWallet},
			},
		}
		for _, filter := range filters {
			var logs []rpcLog
			if err := client.call(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
				return nil, fmt.Errorf("scan blocks %d-%d: %w", start, end, err)
			}
			for _, log := range logs {
				dedup[strings.ToLower(log.TransactionHash)+":"+strings.ToLower(log.LogIndex)] = log
			}
		}
		if end == toBlock {
			break
		}
		start = end + 1
	}

	logs := make([]rpcLog, 0, len(dedup))
	for _, log := range dedup {
		logs = append(logs, log)
	}
	sort.Slice(logs, func(i, j int) bool {
		bi, _ := parseHexUint64(logs[i].BlockNumber)
		bj, _ := parseHexUint64(logs[j].BlockNumber)
		if bi != bj {
			return bi < bj
		}
		li, _ := parseHexUint64(logs[i].LogIndex)
		lj, _ := parseHexUint64(logs[j].LogIndex)
		return li < lj
	})

	payments := make([]Payment, 0, len(logs))
	byTransaction := map[string][]int{}
	for _, log := range logs {
		payment, err := transferPayment(log, wallet)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
		key := strings.ToLower(payment.TransactionHash)
		byTransaction[key] = append(byTransaction[key], len(payments)-1)
	}

	for transactionHash, indices := range byTransaction {
		var receipt rpcReceipt
		if err := client.call(ctx, "eth_getTransactionReceipt", []any{transactionHash}, &receipt); err != nil {
			return nil, fmt.Errorf("read receipt %s: %w", transactionHash, err)
		}
		memos, err := extractMemos(receipt.Logs)
		if err != nil {
			return nil, fmt.Errorf("decode memos in %s: %w", transactionHash, err)
		}
		attachMemos(payments, indices, memos)

		if NormalizeAddress(receipt.From) == wallet && len(indices) > 0 {
			fee, err := receiptFeeMicros(receipt)
			if err != nil {
				return nil, fmt.Errorf("calculate fee for %s: %w", transactionHash, err)
			}
			payments[indices[0]].FeeMicros = fee
		}
	}
	return payments, nil
}

func (client *RPCClient) call(ctx context.Context, method string, params any, target any) error {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	if len(client.URLs) == 0 {
		return fmt.Errorf("no RPC endpoints configured")
	}
	var failures []error
	for _, endpoint := range client.URLs {
		if err := client.callEndpoint(ctx, endpoint, payload, target); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Errorf("%s: %w", endpoint, err))
		}
	}
	return fmt.Errorf("all RPC endpoints failed: %w", errors.Join(failures...))
}

func (client *RPCClient) callEndpoint(ctx context.Context, endpoint string, payload []byte, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	response, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("RPC returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope rpcResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode RPC response: %w", err)
	}
	if envelope.Error != nil {
		return fmt.Errorf("RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if target == nil {
		return nil
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return fmt.Errorf("RPC returned no result")
	}
	return json.Unmarshal(envelope.Result, target)
}

func transferPayment(log rpcLog, wallet string) (Payment, error) {
	if NormalizeAddress(log.Address) != ArcUSDCAddress || len(log.Topics) < 3 || !strings.EqualFold(log.Topics[0], transferTopic) {
		return Payment{}, fmt.Errorf("unexpected transfer log")
	}
	from, err := topicAddress(log.Topics[1])
	if err != nil {
		return Payment{}, err
	}
	to, err := topicAddress(log.Topics[2])
	if err != nil {
		return Payment{}, err
	}
	amount, err := parseHexBig(log.Data)
	if err != nil {
		return Payment{}, fmt.Errorf("parse transfer amount: %w", err)
	}
	if !amount.IsInt64() {
		return Payment{}, fmt.Errorf("transfer amount exceeds supported range")
	}
	block, err := parseHexUint64(log.BlockNumber)
	if err != nil {
		return Payment{}, err
	}
	index, err := parseHexUint64(log.LogIndex)
	if err != nil {
		return Payment{}, err
	}
	direction := DirectionOutbound
	switch {
	case from == wallet && to == wallet:
		direction = DirectionSelf
	case to == wallet:
		direction = DirectionInbound
	case from == wallet:
		direction = DirectionOutbound
	default:
		return Payment{}, fmt.Errorf("transfer does not involve watched wallet")
	}
	return Payment{
		TransactionHash: strings.ToLower(log.TransactionHash),
		BlockNumber:     block,
		LogIndex:        index,
		From:            from,
		To:              to,
		Direction:       direction,
		AmountMicros:    amount.Int64(),
	}, nil
}

func extractMemos(logs []rpcLog) ([]memoData, error) {
	var memos []memoData
	for _, log := range logs {
		if NormalizeAddress(log.Address) != ArcMemoAddress || len(log.Topics) < 4 || !strings.EqualFold(log.Topics[0], memoTopic) {
			continue
		}
		decoded, err := decodeHex(log.Data)
		if err != nil {
			return nil, err
		}
		if len(decoded) < 96 {
			return nil, fmt.Errorf("memo event data is shorter than ABI head")
		}
		offset := new(big.Int).SetBytes(decoded[32:64])
		if !offset.IsInt64() || offset.Int64() < 0 {
			return nil, fmt.Errorf("memo byte offset is invalid")
		}
		start := int(offset.Int64())
		if start+32 > len(decoded) {
			return nil, fmt.Errorf("memo byte offset exceeds event data")
		}
		length := new(big.Int).SetBytes(decoded[start : start+32])
		if !length.IsInt64() || length.Int64() < 0 {
			return nil, fmt.Errorf("memo byte length is invalid")
		}
		end := start + 32 + int(length.Int64())
		if end > len(decoded) {
			return nil, fmt.Errorf("memo bytes exceed event data")
		}
		index, err := parseHexUint64(log.LogIndex)
		if err != nil {
			return nil, err
		}
		memoBytes := decoded[start+32 : end]
		memos = append(memos, memoData{
			ID:       strings.ToLower(log.Topics[3]),
			Text:     displayMemo(memoBytes),
			LogIndex: index,
		})
	}
	sort.Slice(memos, func(i, j int) bool { return memos[i].LogIndex < memos[j].LogIndex })
	return memos, nil
}

func attachMemos(payments []Payment, indices []int, memos []memoData) {
	if len(memos) == 0 || len(indices) == 0 {
		return
	}
	if len(memos) == 1 {
		for _, index := range indices {
			payments[index].MemoID = memos[0].ID
			payments[index].MemoText = memos[0].Text
		}
		return
	}
	if len(memos) == len(indices) {
		for i, index := range indices {
			payments[index].MemoID = memos[i].ID
			payments[index].MemoText = memos[i].Text
		}
	}
}

func receiptFeeMicros(receipt rpcReceipt) (int64, error) {
	gasUsed, err := parseHexBig(receipt.GasUsed)
	if err != nil {
		return 0, err
	}
	gasPrice, err := parseHexBig(receipt.EffectiveGasPrice)
	if err != nil {
		return 0, err
	}
	feeWei := new(big.Int).Mul(gasUsed, gasPrice)
	feeMicros := new(big.Int).Div(feeWei, big.NewInt(1_000_000_000_000))
	if !feeMicros.IsInt64() {
		return 0, fmt.Errorf("fee exceeds supported range")
	}
	return feeMicros.Int64(), nil
}

func topicAddress(topic string) (string, error) {
	bytes, err := decodeHex(topic)
	if err != nil {
		return "", err
	}
	if len(bytes) != 32 {
		return "", fmt.Errorf("address topic must contain 32 bytes")
	}
	return "0x" + hex.EncodeToString(bytes[12:]), nil
}

func displayMemo(value []byte) string {
	if utf8.Valid(value) {
		text := string(value)
		printable := true
		for _, r := range text {
			if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
				printable = false
				break
			}
		}
		if printable {
			return text
		}
	}
	return "0x" + hex.EncodeToString(value)
}

func decodeHex(value string) ([]byte, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if len(value)%2 != 0 {
		value = "0" + value
	}
	return hex.DecodeString(value)
}

func parseHexBig(value string) (*big.Int, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "0x")
	if value == "" {
		return big.NewInt(0), nil
	}
	result := new(big.Int)
	if _, ok := result.SetString(value, 16); !ok {
		return nil, fmt.Errorf("invalid hexadecimal integer %q", value)
	}
	return result, nil
}

func parseHexUint64(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(value), "0x"), 16, 64)
}

func hexUint64(value uint64) string {
	return fmt.Sprintf("0x%x", value)
}
