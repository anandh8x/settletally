import type { Direction, ExpectedInput } from "./types";

const requiredColumns = ["reference", "direction", "amount", "counterparty"] as const;
const addressPattern = /^0x[0-9a-fA-F]{40}$/;

export function parseExpectedCSV(source: string): ExpectedInput[] {
  const rows = parseRows(source);
  if (rows.length === 0) throw new Error("The CSV is empty.");
  const header = rows[0].map((value) => value.trim().toLowerCase());
  const indexes = new Map(header.map((value, index) => [value, index]));
  for (const column of requiredColumns) {
    if (!indexes.has(column)) throw new Error(`Missing required column: ${column}`);
  }

  const records: ExpectedInput[] = [];
  const references = new Set<string>();
  for (let rowIndex = 1; rowIndex < rows.length; rowIndex += 1) {
    const row = rows[rowIndex];
    if (row.every((value) => value.trim() === "")) continue;
    const get = (column: string) => {
      const index = indexes.get(column);
      return index === undefined ? "" : (row[index] ?? "").trim();
    };
    const reference = get("reference");
    if (!reference) throw new Error(`Row ${rowIndex + 1}: reference is required.`);
    const referenceKey = reference.toLowerCase();
    if (references.has(referenceKey)) {
      throw new Error(`Row ${rowIndex + 1}: reference ${reference} is duplicated.`);
    }
    references.add(referenceKey);

    const direction = get("direction") as Direction;
    if (!(["inbound", "outbound", "self"] as string[]).includes(direction)) {
      throw new Error(`Row ${rowIndex + 1}: direction must be inbound, outbound, or self.`);
    }
    const amount = get("amount");
    if (!/^\d+(\.\d{1,6})?$/.test(amount) || Number(amount) <= 0) {
      throw new Error(`Row ${rowIndex + 1}: amount must be positive with at most 6 decimals.`);
    }
    const counterparty = get("counterparty");
    if (!addressPattern.test(counterparty)) {
      throw new Error(`Row ${rowIndex + 1}: counterparty is not a valid address.`);
    }
    const dueDate = get("due_date");
    if (dueDate && !/^\d{4}-\d{2}-\d{2}$/.test(dueDate)) {
      throw new Error(`Row ${rowIndex + 1}: due_date must use YYYY-MM-DD.`);
    }
    records.push({
      reference,
      direction,
      amount,
      counterparty: counterparty.toLowerCase(),
      ...(dueDate ? { dueDate } : {}),
      ...(get("memo_id") ? { memoId: get("memo_id").toLowerCase() } : {}),
    });
  }
  if (records.length === 0) throw new Error("The CSV contains no records.");
  return records;
}

function parseRows(source: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let quoted = false;
  for (let index = 0; index < source.length; index += 1) {
    const character = source[index];
    if (quoted) {
      if (character === '"' && source[index + 1] === '"') {
        field += '"';
        index += 1;
      } else if (character === '"') {
        quoted = false;
      } else {
        field += character;
      }
      continue;
    }
    if (character === '"' && field === "") quoted = true;
    else if (character === ",") {
      row.push(field);
      field = "";
    } else if (character === "\n") {
      row.push(field.replace(/\r$/, ""));
      rows.push(row);
      row = [];
      field = "";
    } else field += character;
  }
  if (quoted) throw new Error("The CSV contains an unclosed quoted field.");
  if (field !== "" || row.length > 0) {
    row.push(field.replace(/\r$/, ""));
    rows.push(row);
  }
  return rows;
}
