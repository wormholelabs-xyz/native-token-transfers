import { describe, it, expect } from "bun:test";
import { U64_MAX } from "../src/constants";

describe("Solana deploy defaults", () => {
  it("U64_MAX equals 2^64 - 1", () => {
    expect(U64_MAX).toBe(2n ** 64n - 1n);
    expect(U64_MAX).toBe(18446744073709551615n);
  });
});
