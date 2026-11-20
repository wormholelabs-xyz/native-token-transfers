import { describe, expect, test } from "bun:test";

import {
  parseCantonPartyId,
  parseEvmAddress,
  parseRawAmount,
  parseSolanaAddressBytes32,
  parseTokenDecimals,
  parseTransferNonceU32,
  parseWormholeChainId,
} from "../types.js";

const FINGERPRINT = "1220" + "ab".repeat(32);

describe("parseCantonPartyId", () => {
  test.each([
    [`alice::${FINGERPRINT}`],
    [`a_b-c::${FINGERPRINT}`],
    [`with space::${FINGERPRINT}`],
    [`a:b::${FINGERPRINT}`],
    [`${"h".repeat(185)}::${FINGERPRINT}`],
  ])("accepts %s", (partyText) => {
    expect(parseCantonPartyId(partyText) as string).toBe(partyText);
  });

  test.each([
    ["bare hint", "alice"],
    ["two separators", `bob::${FINGERPRINT}::${FINGERPRINT}`],
    ["fingerprint prefix 1221", `alice::1221${"ab".repeat(32)}`],
    ["63 hex", `alice::1220${"a".repeat(63)}`],
    ["65 hex", `alice::1220${"a".repeat(65)}`],
    ["uppercase hex", `alice::1220${"AB".repeat(32)}`],
    ["empty hint", `::${FINGERPRINT}`],
    ["dot in hint", `a.b::${FINGERPRINT}`],
    ["slash in hint", `a/b::${FINGERPRINT}`],
    ["trailing colon", `alice:::${FINGERPRINT}`],
    ["186-character hint", `${"h".repeat(186)}::${FINGERPRINT}`],
  ])("rejects %s", (_caseName, partyText) => {
    expect(() => parseCantonPartyId(partyText)).toThrow();
  });
});

describe("parseSolanaAddressBytes32", () => {
  // A 43-character prefix of this key still decodes to 32 bytes; the typed confirmation covers that case.
  const KEY = "HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1Gciw";

  test.each([
    [KEY, "fc12b0dc42b4d8b280cf13e2ee66fb6b2e6a206b5776b3c8ec17830bd35ba524"],
    ["11111111111111111111111111111111", "00".repeat(32)],
  ])("accepts %s", (base58Text, expectedHex) => {
    expect(parseSolanaAddressBytes32(base58Text) as string).toBe(expectedHex);
  });

  test.each([
    ["2 characters removed", KEY.slice(0, -2)],
    ["3 characters removed", KEY.slice(0, -3)],
    ["1 character added", KEY + "z"],
    ["0", KEY.slice(0, -1) + "0"],
    ["O", KEY.slice(0, -1) + "O"],
    ["I", KEY.slice(0, -1) + "I"],
    ["l", KEY.slice(0, -1) + "l"],
    ["empty", ""],
    [
      "64-byte secret key",
      "5MaiiCavjCmn9Hs1o3eznqDEhRwxo7pXiAYez7keQUviUkauRiTMD8DrESdrNjN8zd9mTmVhRvBJeg5vhyvgrAhG",
    ],
  ])("rejects %s", (_caseName, base58Text) => {
    expect(() => parseSolanaAddressBytes32(base58Text)).toThrow();
  });
});

describe("parseEvmAddress", () => {
  const CHECKSUMMED = "0x06803b132816e8F6d277Ad7984305B2F590b038D";

  test("accepts a checksummed address", () => {
    expect(parseEvmAddress(CHECKSUMMED) as string).toBe(CHECKSUMMED);
  });

  test("accepts a lowercase address with the flag", () => {
    expect(
      parseEvmAddress(CHECKSUMMED.toLowerCase(), {
        acceptUnchecksummed: true,
      }) as string
    ).toBe(CHECKSUMMED);
  });

  test.each([
    ["one case flipped", CHECKSUMMED.replace("F", "f")],
    ["lowercase without the flag", CHECKSUMMED.toLowerCase()],
    ["19 bytes", "0x" + "ab".repeat(19)],
    ["no 0x", CHECKSUMMED.slice(2)],
  ])("rejects %s", (_caseName, addressText) => {
    expect(() => parseEvmAddress(addressText)).toThrow();
  });
});

describe("bounded integers", () => {
  test.each([[0], [18]])("decimals %p accepted", (decimals) => {
    expect(parseTokenDecimals(decimals) as number).toBe(decimals);
  });
  test.each([[-1], [19], [1.5], ["8"]])("decimals %p rejected", (decimals) => {
    expect(() => parseTokenDecimals(decimals)).toThrow();
  });
  test.each([[1], [65535]])("chain id %p accepted", (chainId) => {
    expect(parseWormholeChainId(chainId) as number).toBe(chainId);
  });
  test.each([[0], [65536], [1.5]])("chain id %p rejected", (chainId) => {
    expect(() => parseWormholeChainId(chainId)).toThrow();
  });
  test.each([[0], [4294967295], ["7"]])("nonce %p accepted", (nonce) => {
    expect(parseTransferNonceU32(nonce) as number).toBe(Number(nonce));
  });
  test.each([[-1], [4294967296], ["0x1"]])("nonce %p rejected", (nonce) => {
    expect(() => parseTransferNonceU32(nonce)).toThrow();
  });
  test("raw amount 0 rejected, 1 accepted", () => {
    expect(() => parseRawAmount(0n)).toThrow();
    expect(parseRawAmount(1n) as bigint).toBe(1n);
  });
});
