import type { ExpectedInput, Report } from "./types";

interface ReconcileInput {
  wallet: string;
  fromBlock?: number;
  toBlock?: number;
  expectedRecords: ExpectedInput[];
}

export async function reconcileOnArc(input: ReconcileInput, signal?: AbortSignal): Promise<Report> {
  const response = await fetch("/api/v1/reconcile", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
    signal,
  });
  const payload = (await response.json().catch(() => null)) as Report | { error?: string } | null;
  if (!response.ok) {
    throw new Error(payload && "error" in payload && payload.error ? payload.error : "Reconciliation failed.");
  }
  return payload as Report;
}

export async function checkHealth(signal?: AbortSignal): Promise<boolean> {
  try {
    const response = await fetch("/api/health", { signal });
    return response.ok;
  } catch {
    return false;
  }
}
