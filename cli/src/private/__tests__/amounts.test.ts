import { describe, expect, test } from "bun:test";
import { checkDust, fromRaw, toRaw } from "../amounts.js";

describe("toRaw / fromRaw", () => {
  test("whole units at 18 decimals", () => {
    expect(toRaw("1", 18)).toBe(1000000000000000000n);
    expect(fromRaw(1000000000000000000n, 18)).toBe("1");
  });

  test("exact string math avoids float artifacts (0.1 * 1e18 in floats is not exact)", () => {
    expect(toRaw("0.1", 18)).toBe(100000000000000000n);
  });

  test("max-precision string at 10 decimals", () => {
    expect(toRaw("0.0000000001", 10)).toBe(1n);
    expect(fromRaw(1n, 10)).toBe("0.0000000001");
  });

  test("max-precision string at 9 decimals", () => {
    expect(toRaw("0.000000001", 9)).toBe(1n);
    expect(fromRaw(1n, 9)).toBe("0.000000001");
  });

  test("round trip at 8 decimals with trailing-zero trimming", () => {
    const raw = toRaw("123.456789", 8);
    expect(raw).toBe(12345678900n);
    expect(fromRaw(raw, 8)).toBe("123.456789");
  });

  test("trailing zeros beyond decimals are dropped, not rejected", () => {
    expect(toRaw("0.100000000", 8)).toBe(toRaw("0.1", 8));
  });

  test("rejects sub-unit precision that would be lost", () => {
    expect(() => toRaw("0.12345678901", 8)).toThrow(/more precision/);
  });

  test("rejects negative amounts", () => {
    expect(() => toRaw("-1", 18)).toThrow();
  });

  test("rejects non-numeric input", () => {
    expect(() => toRaw("abc", 18)).toThrow();
  });
});

describe("checkDust", () => {
  test("18 -> 10 route rejects sub-1e-8 excess", () => {
    // min(8, 18, 10) = 8, step = 0.00000001; this amount has a 10th-decimal remainder.
    expect(() => checkDust("0.0000000001", 18, 10)).toThrow(
      /multiple of 0\.00000001/
    );
  });

  test("18 -> 10 route accepts the exact step boundary", () => {
    expect(() => checkDust("0.00000001", 18, 10)).not.toThrow();
  });

  test("Canton(10) -> Solana(9) route rejects sub-1e-8 excess", () => {
    // min(8, 10, 9) = 8.
    expect(() => checkDust("0.000000001", 10, 9)).toThrow(
      /multiple of 0\.00000001/
    );
  });

  test("Canton(10) -> Solana(9) route accepts the exact step boundary", () => {
    expect(() => checkDust("0.00000001", 10, 9)).not.toThrow();
  });

  test("whole-unit amounts always pass regardless of decimals", () => {
    expect(() => checkDust("42", 18, 6)).not.toThrow();
  });

  test("low-decimal destination narrows the step below 8", () => {
    // min(8, 18, 2) = 2, step = 0.01.
    expect(() => checkDust("1.001", 18, 2)).toThrow(/multiple of 0\.01/);
    expect(() => checkDust("1.01", 18, 2)).not.toThrow();
  });
});
