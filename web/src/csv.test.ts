import { describe, expect, it } from "vitest";
import { parseExpectedCSV } from "./csv";

describe("parseExpectedCSV", () => {
  it("parses quoted references and optional fields", () => {
    const records = parseExpectedCSV(`reference,direction,amount,counterparty,due_date,memo_id
"INV, 1042",inbound,12.50,0x1111111111111111111111111111111111111111,2026-08-01,0xabc
`);
    expect(records).toEqual([
      {
        reference: "INV, 1042",
        direction: "inbound",
        amount: "12.50",
        counterparty: "0x1111111111111111111111111111111111111111",
        dueDate: "2026-08-01",
        memoId: "0xabc",
      },
    ]);
  });

  it("rejects duplicate references", () => {
    expect(() =>
      parseExpectedCSV(`reference,direction,amount,counterparty
INV-1,inbound,1,0x1111111111111111111111111111111111111111
inv-1,inbound,2,0x2222222222222222222222222222222222222222
`),
    ).toThrow(/duplicated/);
  });

  it("rejects more than six decimal places", () => {
    expect(() =>
      parseExpectedCSV(`reference,direction,amount,counterparty
INV-1,inbound,1.0000001,0x1111111111111111111111111111111111111111
`),
    ).toThrow(/at most 6 decimals/);
  });
});
