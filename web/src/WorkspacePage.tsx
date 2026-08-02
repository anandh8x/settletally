import { useEffect, useMemo, useRef, useState } from "react";
import { checkHealth, reconcileOnArc } from "./api";
import { Brand } from "./Brand";
import { parseExpectedCSV } from "./csv";
import { reportToCSV } from "./export";
import { formatUSDC, shortHash, statusLabel } from "./format";
import type { ExpectedInput, MatchStatus, Report } from "./types";

const demoCSV = `reference,direction,amount,counterparty,memo_id
ARC-DEMO-54787773,self,0.033822,0x9a605c93932f729d0bee62899d1ccadc11a9b4bc,0x922b866a8435fae2e426499b327ce0119ac3a81fb6e38cf4ad32caf5c9a032b4
`;

const sampleCSV = `reference,direction,amount,counterparty,due_date,memo_id
INV-1042,inbound,1250.00,0x1111111111111111111111111111111111111111,2026-08-15,
PAYOUT-88,outbound,420.50,0x2222222222222222222222222222222222222222,2026-08-18,
`;

export function WorkspacePage() {
  const [wallet, setWallet] = useState("");
  const [fromBlock, setFromBlock] = useState("");
  const [toBlock, setToBlock] = useState("");
  const [records, setRecords] = useState<ExpectedInput[]>([]);
  const [fileName, setFileName] = useState("");
  const [error, setError] = useState("");
  const [report, setReport] = useState<Report | null>(null);
  const [running, setRunning] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [serviceState, setServiceState] = useState<"checking" | "online" | "offline">("checking");
  const inputRef = useRef<HTMLInputElement>(null);
  const requestRef = useRef<AbortController | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    void checkHealth(controller.signal).then((online) => setServiceState(online ? "online" : "offline"));
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (new URLSearchParams(window.location.search).get("demo") === "1") loadLiveDemo();
  }, []);

  const expectedTotal = useMemo(
    () => records.reduce((total, record) => total + Number(record.amount || 0), 0),
    [records],
  );

  async function readFile(file: File) {
    setError("");
    setReport(null);
    try {
      if (file.size > 2 * 1024 * 1024) throw new Error("CSV files must be smaller than 2 MB.");
      const parsed = parseExpectedCSV(await file.text());
      if (parsed.length > 2_000) throw new Error("CSV files are limited to 2,000 records.");
      setRecords(parsed);
      setFileName(file.name);
    } catch (caught) {
      setRecords([]);
      setFileName("");
      setError(messageOf(caught));
    }
  }

  function loadLiveDemo() {
    setRecords(parseExpectedCSV(demoCSV));
    setFileName("verified-arc-demo.csv");
    setWallet("0x9a605c93932f729d0bee62899d1ccadc11a9b4bc");
    setFromBlock("54787773");
    setToBlock("54787773");
    setReport(null);
    setError("");
  }

  function downloadTemplate() {
    downloadText("settletally-template.csv", sampleCSV, "text/csv;charset=utf-8");
  }

  async function runReconciliation() {
    setError("");
    setReport(null);
    if (!/^0x[0-9a-fA-F]{40}$/.test(wallet.trim())) {
      setError("Enter a valid Arc wallet address.");
      return;
    }
    if (records.length === 0) {
      setError("Import at least one expected record before running reconciliation.");
      return;
    }
    setRunning(true);
    const controller = new AbortController();
    requestRef.current = controller;
    try {
      const result = await reconcileOnArc({
        wallet: wallet.trim(),
        ...(fromBlock ? { fromBlock: Number(fromBlock) } : {}),
        ...(toBlock ? { toBlock: Number(toBlock) } : {}),
        expectedRecords: records,
      }, controller.signal);
      setReport(result);
      requestAnimationFrame(() => document.getElementById("results")?.scrollIntoView({ behavior: "smooth" }));
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === "AbortError") setError("The reconciliation scan was canceled.");
      else setError(messageOf(caught));
    } finally {
      setRunning(false);
      requestRef.current = null;
    }
  }

  return (
    <div className="application-shell">
      <header className="app-header">
        <div className="app-header-left"><Brand /><span className="header-divider" /><span className="workspace-label">Reconciliation workspace</span></div>
        <div className="app-header-right">
          <div className={`network-status ${serviceState}`}><span /> {serviceState === "online" ? "Arc service ready" : serviceState === "offline" ? "Service offline" : "Checking service"}</div>
          <a href="/">Product overview <span>↗</span></a>
        </div>
      </header>

      <main className="application-main">
        <section className="workspace-intro">
          <div><p className="kicker">Read-only operations</p><h1>Reconcile Arc USDC.</h1></div>
          <div><p>Compare expected business records against settled Arc activity. Confirm exact matches and isolate only what requires review.</p><button type="button" onClick={loadLiveDemo}>Load verified Arc record <span>↘</span></button></div>
        </section>

        <section className="work-grid" aria-label="Reconciliation inputs">
          <div className="work-panel import-panel">
            <div className="panel-heading"><span>01</span><div><h2>Expected records</h2><p>Invoices, receivables, or payouts in CSV format.</p></div></div>
            <div
              className={`dropzone ${dragging ? "is-dragging" : ""} ${records.length ? "has-file" : ""}`}
              onDragEnter={(event) => { event.preventDefault(); setDragging(true); }}
              onDragOver={(event) => event.preventDefault()}
              onDragLeave={() => setDragging(false)}
              onDrop={(event) => {
                event.preventDefault();
                setDragging(false);
                const file = event.dataTransfer.files[0];
                if (file) void readFile(file);
              }}
            >
              <input ref={inputRef} type="file" accept=".csv,text/csv" aria-label="Import expected records CSV" onChange={(event) => { const file = event.target.files?.[0]; if (file) void readFile(file); }} />
              <div className="upload-mark" aria-hidden="true">↑</div>
              {records.length ? <><strong>{fileName}</strong><p>{records.length} {records.length === 1 ? "record" : "records"} ready · {expectedTotal.toLocaleString("en-US", { maximumFractionDigits: 6 })} USDC expected</p></> : <><strong>Import expected records</strong><p>Drop a CSV here or select one from your computer.</p></>}
              <button type="button" onClick={() => inputRef.current?.click()}>{records.length ? "Replace CSV" : "Choose CSV"}</button>
            </div>
            <button className="quiet-link" type="button" onClick={downloadTemplate}>Download CSV template <span>↓</span></button>
            {records.length > 0 && <RecordPreview records={records} />}
          </div>

          <div className="work-panel account-panel">
            <div className="panel-heading"><span>02</span><div><h2>Arc activity</h2><p>The account and history to compare.</p></div></div>
            <label className="field"><span>ARC WALLET ADDRESS</span><input value={wallet} onChange={(event) => setWallet(event.target.value)} placeholder="0x..." spellCheck={false} /></label>
            <div className="field-row">
              <label className="field"><span>FROM BLOCK <small>OPTIONAL</small></span><input type="number" min="0" value={fromBlock} onChange={(event) => setFromBlock(event.target.value)} placeholder="Latest − 500" /></label>
              <label className="field"><span>TO BLOCK <small>OPTIONAL</small></span><input type="number" min="0" value={toBlock} onChange={(event) => setToBlock(event.target.value)} placeholder="Latest" /></label>
            </div>
            <div className="privacy-note"><span>◇</span><p><strong>Read-only by design</strong>Public Arc events only. No signature, private key, or custody.</p></div>
            <button className="run-button" type="button" onClick={() => running ? requestRef.current?.abort() : void runReconciliation()}>
              {running ? <><span className="spinner" /> Cancel Arc scan</> : <>Run reconciliation <span>→</span></>}
            </button>
            <button className="demo-link" type="button" onClick={loadLiveDemo}>Use the live Arc test record</button>
          </div>
        </section>

        {error && <div className="error-banner" role="alert"><span>!</span><p><strong>Could not continue</strong>{error}</p></div>}
        {report && <Results report={report} />}
      </main>

      <footer className="app-footer"><span>SettleTally on Arc Testnet</span><span>Public onchain data remains public</span><a href="https://testnet.arcscan.app" target="_blank" rel="noreferrer">ArcScan ↗</a></footer>
    </div>
  );
}

function RecordPreview({ records }: { records: ExpectedInput[] }) {
  return <div className="record-preview"><div className="record-preview-head"><span>REFERENCE</span><span>DIRECTION</span><span>AMOUNT</span></div>{records.slice(0, 4).map((record) => <div className="record-preview-row" key={record.reference}><strong>{record.reference}</strong><span>{record.direction}</span><span>{record.amount} USDC</span></div>)}{records.length > 4 && <p>+ {records.length - 4} more records</p>}</div>;
}

function Results({ report }: { report: Report }) {
  const [filter, setFilter] = useState<"all" | "matched" | "attention">("all");
  const exceptions = report.matches.filter((match) => match.status !== "matched" && match.status !== "awaiting_payment");
  const awaiting = report.matches.filter((match) => match.status === "awaiting_payment").length;
  const matched = report.matches.filter((match) => match.status === "matched").length;
  const visibleMatches = report.matches.filter((match) => {
    if (filter === "matched") return match.status === "matched";
    if (filter === "attention") return match.status !== "matched" && match.status !== "awaiting_payment";
    return true;
  });
  const fileStem = `settletally-${report.fromBlock}-${report.toBlock}`;
  const exportJSON = () => downloadText(`${fileStem}.json`, JSON.stringify(report, null, 2), "application/json");
  const exportCSV = () => downloadText(`${fileStem}.csv`, reportToCSV(report), "text/csv;charset=utf-8");
  return <section className="results" id="results">
    <div className="results-head"><div><p className="kicker">Reconciliation complete</p><h2>{exceptions.length === 0 && report.unmatchedPayments.length === 0 ? awaiting > 0 ? "No unexpected exceptions." : "Everything accounted for." : "Attention has a clear address."}</h2><p>Blocks {report.fromBlock.toLocaleString()} to {report.toBlock.toLocaleString()} · generated {new Date(report.generatedAt).toLocaleString()}</p></div><div className="export-actions"><button type="button" onClick={exportCSV}>Export CSV ↓</button><button type="button" onClick={exportJSON}>Export JSON ↓</button></div></div>
    <div className="metric-grid">
      <Metric label="EXPECTED" value={formatUSDC(report.expectedMicros)} note={`${report.expectedCount} ${report.expectedCount === 1 ? "record" : "records"}`} />
      <Metric label="RECONCILED" value={formatUSDC(report.matchedMicros)} note={`${matched} exact ${matched === 1 ? "match" : "matches"}`} tone="good" />
      <Metric label="REVIEW VALUE" value={formatUSDC(report.reviewMicros)} note={`${exceptions.length} need attention · ${awaiting} awaiting`} tone="warn" />
      <Metric label="UNMATCHED" value={formatUSDC(report.unmatchedMicros)} note={`${report.unmatchedPayments.length} Arc transfers`} tone="bad" />
      <Metric label="ARC FEES" value={formatUSDC(report.feeMicros)} note="paid by watched wallet" />
    </div>
    <div className="result-table-wrap">
      <div className="table-title"><h3>Record outcomes</h3><div className="table-filters" aria-label="Filter record outcomes"><button className={filter === "all" ? "active" : ""} type="button" onClick={() => setFilter("all")}>All {report.matches.length}</button><button className={filter === "matched" ? "active" : ""} type="button" onClick={() => setFilter("matched")}>Matched {matched}</button><button className={filter === "attention" ? "active" : ""} type="button" onClick={() => setFilter("attention")}>Attention {exceptions.length}</button></div></div>
      <div className="result-table" role="table" aria-label="Reconciliation outcomes">
        <div className="result-row result-header" role="row"><span>REFERENCE</span><span>EXPECTED</span><span>SETTLED</span><span>STATUS</span><span>WHY</span></div>
        {visibleMatches.map((match) => <div className="result-row" role="row" key={match.expected.reference}><strong>{match.expected.reference}<small>{shortHash(match.expected.counterparty)}</small></strong><span>{formatUSDC(match.expected.amountMicros)}</span><span>{formatUSDC(match.matchedMicros)}</span><StatusBadge status={match.status} /><span className="reason">{match.reason}{match.payments[0] && <a href={`https://testnet.arcscan.app/tx/${match.payments[0].transactionHash}`} target="_blank" rel="noreferrer">View transaction ↗</a>}</span></div>)}
      </div>
    </div>
    {report.unmatchedPayments.length > 0 && <div className="unmatched"><div className="table-title"><h3>Unmatched Arc activity</h3><span>NO EXPECTED RECORD</span></div>{report.unmatchedPayments.map((payment) => <a key={`${payment.transactionHash}:${payment.logIndex}`} href={`https://testnet.arcscan.app/tx/${payment.transactionHash}`} target="_blank" rel="noreferrer"><span>{shortHash(payment.transactionHash)}</span><strong>{formatUSDC(payment.amountMicros)} USDC</strong><span>{payment.memoText || "No readable memo"}</span><b>↗</b></a>)}</div>}
  </section>;
}

function Metric({ label, value, note, tone = "" }: { label: string; value: string; note: string; tone?: string }) {
  return <div className={`metric ${tone}`}><span>{label}</span><strong>{value}<small> USDC</small></strong><p>{note}</p></div>;
}

function StatusBadge({ status }: { status: MatchStatus }) {
  return <span className={`status status-${status}`}>{statusLabel(status)}</span>;
}

function messageOf(value: unknown): string {
  return value instanceof Error ? value.message : "An unexpected error occurred.";
}

function downloadText(name: string, content: string, type: string) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  anchor.click();
  URL.revokeObjectURL(url);
}
