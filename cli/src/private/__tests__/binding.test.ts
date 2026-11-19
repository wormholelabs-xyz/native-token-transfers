import { describe, expect, test } from "bun:test";
import { base58ToHex32, pad32, recipientBinding } from "../binding.js";

const NAMESPACE =
  "122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e";

describe("recipientBinding", () => {
  // Golden vectors captured from `recipient_binding` on vm-guardian-private-01.
  test("alice", () => {
    expect(recipientBinding(`alice::${NAMESPACE}`)).toBe(
      "0xb57bfe41337237d696834be81e491c78099971fc2bc1ad6eedd032583350a516"
    );
  });

  test("operator", () => {
    expect(recipientBinding(`operator::${NAMESPACE}`)).toBe(
      "0xefd963b986b976980dc83bfc06974761a4fb4498683abe194242da93222e4051"
    );
  });
});

describe("pad32", () => {
  // Externally known relationship: pad32 of the canburn Ethereum transceiver
  // equals the canburn Ethereum emitter recorded in the registry.
  test("canburn Ethereum transceiver -> canburn Ethereum emitter", () => {
    expect(pad32("0x06803b132816e8F6d277Ad7984305B2F590b038D")).toBe(
      "00000000000000000000000006803b132816e8f6d277ad7984305b2f590b038d"
    );
  });

  test("rejects input that is not a 20-byte address", () => {
    expect(() => pad32("0x1234")).toThrow();
  });
});

describe("base58ToHex32", () => {
  // Expected hex independently computed (python base58 decode), not derived
  // from this implementation.
  test("canburn Solana mint", () => {
    expect(base58ToHex32("HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1Gciw")).toBe(
      "fc12b0dc42b4d8b280cf13e2ee66fb6b2e6a206b5776b3c8ec17830bd35ba524"
    );
  });

  test("canhub Solana manager", () => {
    expect(base58ToHex32("chubzpSA92ErGkSKVKH879EqpnLgfLyR3dPpRRnw71a")).toBe(
      "092594fb46f02866246771e2786ffbb9c230db9cfe4ae65db3a1c996975fcfb9"
    );
  });

  test("rejects invalid base58 alphabet", () => {
    expect(() => base58ToHex32("not-base58!")).toThrow();
  });

  test("rejects empty input", () => {
    expect(() => base58ToHex32("")).toThrow();
  });
});
