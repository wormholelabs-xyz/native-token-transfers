import { describe, expect, test } from "bun:test";

import {
  checkAmountHasNoDust,
  formatRawAmount,
  parseHumanAmount,
} from "../amounts.js";
import { parseRawAmount, parseTokenDecimals } from "../types.js";

const decimals = (value: number) => parseTokenDecimals(value);

describe("parseHumanAmount", () => {
  test.each([
    ["1", 8, 100000000n],
    ["1.5", 8, 150000000n],
    ["0.00000001", 8, 1n],
    ["1.10", 1, 11n],
    ["7", 0, 7n],
  ])("%s at %d decimals -> %p", (human, decimalCount, expectedRaw) => {
    expect(parseHumanAmount(human, decimals(decimalCount)) as bigint).toBe(
      expectedRaw
    );
  });

  test.each([
    ["0", 8],
    ["0.0", 8],
    ["0.0000000000", 10],
    ["-1", 8],
    ["1e3", 8],
    ["1.123", 2],
    ["", 8],
    [".5", 8],
  ])("rejects %p at %d decimals", (human, decimalCount) => {
    expect(() => parseHumanAmount(human, decimals(decimalCount))).toThrow();
  });
});

describe("formatRawAmount", () => {
  test.each([
    [150000000n, 8, "1.5"],
    [1n, 8, "0.00000001"],
    [0n, 8, "0"],
    [7n, 0, "7"],
  ])("%p at %d -> %s", (raw, decimalCount, expected) => {
    expect(formatRawAmount(raw, decimalCount)).toBe(expected);
  });
  test("rejects a negative value", () => {
    expect(() => formatRawAmount(-1n, 8)).toThrow();
  });
});

describe("checkAmountHasNoDust", () => {
  test("accepts a multiple of the 8-decimal step", () => {
    expect(() =>
      checkAmountHasNoDust(
        parseRawAmount(10n ** 10n),
        decimals(18),
        decimals(10)
      )
    ).not.toThrow();
  });
  test.each([
    [1n, 18, 10],
    [10n ** 10n + 1n, 18, 18],
    [5n, 10, 9],
  ])(
    "rejects %p from %d to %d decimals",
    (raw, sourceDecimals, destinationDecimals) => {
      expect(() =>
        checkAmountHasNoDust(
          parseRawAmount(raw),
          decimals(sourceDecimals),
          decimals(destinationDecimals)
        )
      ).toThrow(/multiple of/);
    }
  );
});
