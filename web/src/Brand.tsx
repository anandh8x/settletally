export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <a className={`brand ${compact ? "brand-compact" : ""}`} href="/" aria-label="SettleTally home">
      <span className="brand-seal" aria-hidden="true"><i /><i /><i /></span>
      <span>SettleTally</span>
    </a>
  );
}
