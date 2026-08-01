import { Brand } from "./Brand";

export function LandingPage() {
  return (
    <div className="arc-landing">
      <div className="arc-orbits" aria-hidden="true"><i /><i /><i /></div>
      <header className="landing-header">
        <Brand />
        <nav aria-label="Primary navigation"><a href="#product">Product</a><a href="#workflow">Workflow</a><a href="#arc">Why Arc</a></nav>
        <a className="landing-launch" href="/app">Launch workspace <span>↗</span></a>
      </header>

      <main>
        <section className="landing-hero">
          <p><span /> LIVE ON ARC TESTNET</p>
          <h1>Know what settled.<br />Know what did not.</h1>
          <p className="landing-lede">A clear operational layer between expected business records and public USDC settlement on Arc.</p>
          <div className="landing-actions"><a href="/app">Start a reconciliation <span>→</span></a><a href="/app?demo=1">Inspect live Arc data</a></div>
        </section>

        <section className="landing-proof" id="product">
          <div className="proof-bar"><span>RECONCILIATION REPORT</span><b><i /> ARC SERVICE READY</b></div>
          <div className="proof-score"><div><span>CONFIRMED</span><strong>95.4%</strong><small>$23,690.00 settled</small></div><div><span>EXPECTED</span><strong>18</strong><small>business records</small></div><div><span>ATTENTION</span><strong>02</strong><small>exceptions isolated</small></div></div>
          <div className="proof-records"><div className="proof-record-head"><span>REFERENCE</span><span>COUNTERPARTY</span><span>SETTLED</span><span>RESULT</span></div><ProofRow reference="INV-1042" address="0x71a4…a91e" amount="$12,500.00" status="Matched" /><ProofRow reference="PAY-0088" address="0x0d83…4bc2" amount="$8,340.00" status="Matched" /><ProofRow reference="INV-1059" address="0x9fe1…d023" amount="$1,150.00" status="Review" attention /></div>
          <div className="proof-foot"><span>Illustrative interface preview</span><a href="/app">Open the working application ↗</a></div>
        </section>

        <section className="landing-flow" id="workflow">
          <div><span>01</span><h2>Expected records in</h2><p>Import validated invoices, receivables, or payout obligations from CSV.</p></div>
          <div><span>02</span><h2>Arc events observed</h2><p>Read canonical USDC transfers, transaction memos, block positions, and fees.</p></div>
          <div><span>03</span><h2>Exceptions out</h2><p>Confirm strong matches and send every ambiguous candidate to review.</p></div>
        </section>

        <section className="landing-arc" id="arc">
          <div><p>BUILT ON ARC'S EXISTING PRIMITIVES</p><h2>Settlement data becomes operational evidence.</h2></div>
          <div><p>SettleTally is read-only. It indexes Arc's canonical USDC Transfer events and transaction Memo events without deploying another contract, connecting a wallet, or taking custody.</p><div><span><b>5042002</b>Chain ID</span><span><b>6</b>USDC decimals</span><span><b>0</b>Signatures</span></div></div>
        </section>
      </main>

      <footer className="landing-footer"><Brand compact /><p>Working Arc Testnet product. Public onchain data remains public.</p><div><a href="https://github.com/anandh8x/settletally" target="_blank" rel="noreferrer">Source ↗</a><a href="https://testnet.arcscan.app" target="_blank" rel="noreferrer">ArcScan ↗</a></div></footer>
    </div>
  );
}

function ProofRow({ reference, address, amount, status, attention = false }: { reference: string; address: string; amount: string; status: string; attention?: boolean }) {
  return <div className={`proof-record ${attention ? "attention" : ""}`}><strong>{reference}</strong><span>{address}</span><span>{amount}</span><b><i />{status}</b></div>;
}
