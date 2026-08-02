import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const liveReport = {
  generatedAt: "2026-08-01T00:00:00Z",
  wallet: "0x9a605c93932f729d0bee62899d1ccadc11a9b4bc",
  fromBlock: 54787773,
  toBlock: 54787773,
  expectedCount: 1,
  paymentCount: 1,
  expectedMicros: 33822,
  matchedMicros: 33822,
  reviewMicros: 0,
  unmatchedMicros: 0,
  feeMicros: 1533,
  matches: [{
    expected: {
      reference: "ARC-DEMO-54787773",
      direction: "self",
      amountMicros: 33822,
      counterparty: "0x9a605c93932f729d0bee62899d1ccadc11a9b4bc",
      memoId: "0x922b866a8435fae2e426499b327ce0119ac3a81fb6e38cf4ad32caf5c9a032b4",
    },
    payments: [{
      transactionHash: "0xaaaa0c0983f1e7007488d581255fd582cf74993e863117cc1033aca8e47c92e4",
      blockNumber: 54787773,
      logIndex: 4,
      from: "0x9a605c93932f729d0bee62899d1ccadc11a9b4bc",
      to: "0x9a605c93932f729d0bee62899d1ccadc11a9b4bc",
      direction: "self",
      amountMicros: 33822,
      feeMicros: 1533,
      memoText: "pizza fund",
    }],
    status: "matched",
    matchedMicros: 33822,
    differenceMicros: 0,
    reason: "exact Arc memo ID",
    confidence: "high",
  }],
  unmatchedPayments: [],
};

beforeEach(() => {
  window.history.replaceState({}, "", "/app");
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  window.history.replaceState({}, "", "/");
});

describe("SettleTally workspace", () => {
  it("loads the live record and presents the reconciliation result", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(JSON.stringify({ status: "ok" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(liveReport), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    Element.prototype.scrollIntoView = vi.fn();
    const user = userEvent.setup();
    render(<App />);

    await waitFor(() => expect(screen.getByText("Arc service ready")).toBeTruthy());
    await user.click(screen.getByRole("button", { name: "Use the live Arc test record" }));
    expect(screen.getByText("verified-arc-demo.csv")).toBeTruthy();
    expect(screen.getByText(/1 record ready/)).toBeTruthy();
    expect(screen.getAllByDisplayValue("54787773")).toHaveLength(2);

    await user.click(screen.getByRole("button", { name: /Run reconciliation/ }));
    await waitFor(() => expect(screen.getByText("Everything accounted for.")).toBeTruthy());
    expect(screen.getAllByText("ARC-DEMO-54787773").length).toBeGreaterThan(0);
    expect(screen.getByText("exact Arc memo ID")).toBeTruthy();
    expect(fetchMock).toHaveBeenLastCalledWith("/api/v1/reconcile", expect.objectContaining({ method: "POST" }));
  });

  it("blocks a run until records and a valid address are present", async () => {
    vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(new Response("{}", { status: 200 })));
    const user = userEvent.setup();
    render(<App />);
    await user.click(screen.getByRole("button", { name: /Run reconciliation/ }));
    expect(screen.getByRole("alert").textContent).toContain("Enter a valid Arc wallet address");
  });
});

describe("SettleTally landing page", () => {
  it("directs visitors into the dedicated application", () => {
    window.history.replaceState({}, "", "/");
    render(<App />);

    expect(screen.getByRole("heading", { name: "Know what settled. Know what did not." })).toBeTruthy();
    expect(screen.getByRole("link", { name: /Start a reconciliation/ }).getAttribute("href")).toBe("/app");
    expect(screen.queryByRole("button", { name: /Run reconciliation/ })).toBeNull();
  });
});
