# SettleTally

SettleTally reconciles expected business records against settled USDC activity on Arc. It imports invoices, receivables, or payout records from CSV, reads public Arc events, applies deterministic matching rules, and produces an explainable exception report.

The product is read-only. It does not connect a wallet, request a signature, hold funds, or deploy a custom smart contract.

## The problem

A block explorer can show that a transfer happened, but a finance team still needs to know which invoice or payout it belongs to. Similar amounts, missing references, partial payments, duplicate transfers, and incorrect counterparties turn that mapping into manual spreadsheet work.

Arc provides two useful primitives for solving this cleanly:

- a canonical six-decimal USDC interface whose `Transfer` events represent application-level settlement;
- transaction memos that can carry an invoice ID or another business reference alongside a forwarded call.

SettleTally combines those public signals with the records a business expected to settle. It does not treat a weak guess as a confirmed match.

## How it works

1. Import a CSV containing the records that should have been paid or received.
2. Enter the Arc account and an optional block range.
3. SettleTally reads USDC `Transfer` events involving that account.
4. It reads transaction receipts, decodes Arc `Memo` events, and calculates wallet-paid gas fees.
5. Ordered matching rules compare each expected record with the observed payments.
6. The interface separates confirmed matches from partial, excess, missing, ambiguous, and unmatched activity.
7. Export the complete result as JSON or a spreadsheet-safe CSV.

No imported records are written to a database by this application. They are sent to the stateless reconciliation endpoint for the duration of one request and then discarded.

## Matching rules

Rules are applied in order. A payment is assigned only once.

| Priority | Signal | Result |
| --- | --- | --- |
| 1 | Exact Arc memo ID and direction | High-confidence candidate |
| 2 | Exact memo text reference and direction | High-confidence candidate |
| 3 | Unique amount, counterparty, and direction | High-confidence candidate |
| 4 | Multiple identical candidates | Manual review |
| 5 | Counterparty-only partial candidates | Manual review |
| 6 | Unique amount without the expected counterparty | Manual review |
| 7 | No candidate before the due date | Awaiting payment |
| 8 | No candidate after the due date | Missing payment |

An exact memo with the wrong counterparty is always marked for review. Confirmed totals exclude anything in review. Overpayments contribute no more than the expected amount to the reconciled total.

## Reconciliation statuses

- `matched`: the observed amount equals the expected amount using a strong matching signal;
- `partial`: a strong memo signal links one or more payments whose total is below the expected amount;
- `overpaid`: a strong signal links payments whose total exceeds the expected amount;
- `needs_review`: the candidate is ambiguous or the counterparty conflicts with the record;
- `awaiting_payment`: no match exists, but the due date has not passed;
- `missing_payment`: no match exists and the record is past due or has no due date;
- `unmatched`: an Arc payment was observed without a corresponding expected record.

## CSV format

Required columns:

```csv
reference,direction,amount,counterparty
INV-1042,inbound,1250.00,0x1111111111111111111111111111111111111111
PAYOUT-88,outbound,420.50,0x2222222222222222222222222222222222222222
```

Optional columns:

```csv
reference,direction,amount,counterparty,due_date,memo_id
INV-1042,inbound,1250.00,0x1111111111111111111111111111111111111111,2026-08-15,0x...
```

Rules:

- `direction` must be `inbound`, `outbound`, or `self`;
- `amount` must be positive and use no more than six decimal places;
- `counterparty` must be an EVM address;
- `due_date`, when present, must use `YYYY-MM-DD`;
- references are required and must be unique without regard to letter case;
- files are limited to 2 MB and 2,000 records in the hosted application.

## Run locally

Requirements:

- Go 1.26 or newer
- Node.js 22
- pnpm 11

Install the frontend dependencies:

```bash
pnpm install
```

Start the Go API:

```bash
pnpm dev:api
```

In a second terminal, start the web application:

```bash
pnpm dev:web
```

Open `http://127.0.0.1:5173`. If that port is occupied, Vite prints the next available local address.

The interface includes a verified Arc Testnet record that can be loaded without preparing a CSV.

## Command-line usage

The same engine can generate reports without the web interface:

```bash
go run ./cmd/settletally \
  --wallet 0xYOUR_ARC_ADDRESS \
  --expected ./expected.csv \
  --from-block 54787700 \
  --to-block 54788200 \
  --out ./out
```

The command writes `reconciliation.json` and `reconciliation.csv`.

## Configuration

All variables are optional. Defaults target Arc Testnet.

| Variable | Default | Purpose |
| --- | --- | --- |
| `ARC_RPC_URLS` | Arc public RPC failover list | Comma-separated endpoints |
| `ARC_RPC_BATCH_SIZE` | `250` | Blocks requested per log query |
| `MAX_BLOCK_SPAN` | `5000` | Maximum blocks per reconciliation |
| `MAX_EXPECTED_RECORDS` | `2000` | Maximum records per request |
| `MAX_CONCURRENT_SCANS` | `4` | Concurrent scans per process |
| `REQUEST_TIMEOUT_SECONDS` | `45` | Arc request deadline |
| `ALLOWED_ORIGINS` | local Vite origins | Additional cross-origin frontends |

See [`.env.example`](.env.example) for a complete example.

## API

### `GET /api/health`

Returns the service and network state without making an Arc RPC request.

### `POST /api/v1/reconcile`

```json
{
  "wallet": "0x...",
  "fromBlock": 54787700,
  "toBlock": 54788200,
  "expectedRecords": [
    {
      "reference": "INV-1042",
      "direction": "inbound",
      "amount": "1250.00",
      "counterparty": "0x...",
      "dueDate": "2026-08-15",
      "memoId": "0x..."
    }
  ]
}
```

When block numbers are omitted, the API scans the latest 500 blocks.

## Architecture

- React and TypeScript provide CSV validation, workflow controls, result filtering, and exports.
- Go owns exact USDC amount parsing, Arc RPC access, memo decoding, fee calculation, matching, and report generation.
- The API is stateless and bounded by request size, record count, block range, concurrency, and time limits.
- RPC endpoints are tried in order, allowing a scan to recover from a provider failure or rate limit.
- Vercel packages the frontend as static assets and the two API routes as Go Functions.

## Arc integration

SettleTally currently targets:

| Item | Value |
| --- | --- |
| Network | Arc Testnet |
| Chain ID | `5042002` |
| USDC interface | `0x3600000000000000000000000000000000000000` |
| Memo contract | `0x5294E9927c3306DcBaDb03fe70b92e01cCede505` |
| Explorer | `https://testnet.arcscan.app` |

The USDC interface uses six decimals. Native gas accounting uses 18 decimals, so SettleTally converts receipt fees separately before displaying them as USDC.

SettleTally does not deploy a custom contract. Its purpose is to make Arc settlement data operationally useful by indexing Arc's existing USDC and Memo primitives.

Official references:

- [Connect to Arc](https://docs.arc.io/arc/references/connect-to-arc)
- [Arc contract addresses](https://docs.arc.io/arc/references/contract-addresses)
- [Index Arc events](https://docs.arc.io/integrate/infrastructure/indexing-events)
- [Arc gas and fees](https://docs.arc.io/arc/references/gas-and-fees)

## Verification

Run the complete local verification suite:

```bash
pnpm verify
GOCACHE=/tmp/settletally-go-cache go test -race ./...
GOCACHE=/tmp/settletally-go-cache go vet ./...
```

Coverage includes exact amount parsing, CSV validation, RPC failover, transfer and memo decoding, fee conversion, matching outcomes, API validation and error handling, CSV injection protection, and the primary interface workflow.

## Current boundaries

- Arc is currently available on Testnet, so this application uses testnet data only.
- The public application scans at most 5,000 blocks per request. A production backfill service would use a persistent indexer.
- Memo association is conservative for transactions containing multiple transfers. Ambiguous batches remain unassigned rather than being guessed.
- Expected business records are supplied by the user. SettleTally verifies settlement and matching behavior, not whether an invoice itself is legitimate.
- Reports are reconciliation aids, not accounting, tax, legal, or compliance advice.
- Mainnet support requires updating the network configuration after Arc publishes official mainnet endpoints and addresses.

## License

[MIT](LICENSE)
