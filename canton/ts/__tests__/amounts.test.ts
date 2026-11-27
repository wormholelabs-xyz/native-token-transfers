import {
  checkTransferAmount,
  redeemCommandId,
  sendableAmount,
  transferCommandId,
  trimAmount,
  untrimAmount,
  vaaDigest,
} from "../src/index.js";
import { encoding } from "@wormhole-foundation/sdk-base";
import { deserialize } from "@wormhole-foundation/sdk-definitions";

describe("trimming (Amount.daml trimAmountTo, untrimAmount)", () => {
  it.each([
    // raw, from, to, trimmed amount, trimmed decimals
    [123_456_789_012n, 10, 18, 1_234_567_890n, 8],
    [123_456_789_012n, 10, 6, 12_345_678n, 6],
    [500n, 2, 18, 500n, 2],
    [5n, 0, 8, 5n, 0],
    [123_456_789n, 8, 8, 123_456_789n, 8],
  ])(
    "trims %s at %s decimals for a peer with %s",
    (raw, from, to, amount, decimals) => {
      expect(trimAmount(raw, from, to)).toEqual({ amount, decimals });
    }
  );

  it("untrims, truncating when the target has fewer decimals", () => {
    expect(untrimAmount({ amount: 150_000_000n, decimals: 8 }, 10)).toBe(
      15_000_000_000n
    );
    expect(untrimAmount({ amount: 123n, decimals: 8 }, 6)).toBe(1n);
  });

  it("gives the sendable amount without dust", () => {
    expect(sendableAmount(12_345_678_901n, 10, 18)).toBe(12_345_678_900n);
    expect(sendableAmount(12_345_678_900n, 10, 18)).toBe(12_345_678_900n);
    expect(sendableAmount(1_234_567n, 8, 6)).toBe(1_234_500n);
  });
});

describe("checkTransferAmount", () => {
  it("returns the wire amount of a clean amount", () => {
    expect(checkTransferAmount(15_000_000_000n, 10, 18)).toEqual({
      amount: 150_000_000n,
      decimals: 8,
    });
  });

  it("refuses dust, zero after trimming and non-positive amounts", () => {
    expect(() => checkTransferAmount(12_345_678_901n, 10, 18)).toThrow(
      "The transfer amount 12345678901 (at 10 decimals) has dust the peer cannot represent: the step is 10^2 units; send 12345678900 instead"
    );
    expect(() => checkTransferAmount(99n, 10, 18)).toThrow(
      /is zero after trimming to 8 decimals/
    );
    expect(() => checkTransferAmount(0n, 8, 8)).toThrow(/must be positive/);
    expect(() => checkTransferAmount(10n, 8, 256)).toThrow(
      /toDecimals must be an integer/
    );
  });
});

describe("command ids, as the ntt CLI derives them", () => {
  const base = {
    user: "alice::122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e",
    manager: "d03c78c99b240126e5e18bf07bae40847bb41b345b1850c71afb75e809f24d08",
    rawAmount: 5n,
    recipientChainId: 2,
    recipient: "22".repeat(32),
    nonce: 1,
  };

  // Vectors computed with the ntt CLI's former transferCommandId (ethers' ABI encoder).
  it("matches the CLI vectors", () => {
    expect(transferCommandId(base)).toBe(
      "ntt-private-send-84eba7587ff07481f85642ed6c3e9d1e290ddc05b7d79d4c0d7a8707dcbb2f4e"
    );
    expect(
      transferCommandId({
        ...base,
        user: "a-much-longer-party-hint-that-spans-more-than-one-abi-word::122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e",
        rawAmount: 123456789012345678901234567890n,
        nonce: 4294967295,
      })
    ).toBe(
      "ntt-private-send-40426fd261b0288ba04a51fcccc0c9b02e10ea06244d240099ad151bb5bac837"
    );
  });

  it("changes with each field and checks their ranges", () => {
    const id = transferCommandId(base);
    expect(transferCommandId({ ...base, nonce: 2 })).not.toBe(id);
    expect(transferCommandId({ ...base, rawAmount: 6n })).not.toBe(id);
    expect(transferCommandId({ ...base, recipient: "23".repeat(32) })).not.toBe(
      id
    );
    expect(() => transferCommandId({ ...base, nonce: -1 })).toThrow(/uint32/);
    expect(() =>
      transferCommandId({ ...base, recipientChainId: 65536 })
    ).toThrow(/uint16/);
    expect(() =>
      transferCommandId({ ...base, manager: "0x" + base.manager })
    ).toThrow(/64 lowercase hex/);
  });

  it("derives the redeem id and the VAA digest as the CLI does", () => {
    // ETH_TRANSFER_VAA of cli/src/private/__tests__/fixtures/vaas.ts, and its CLI digest.
    const vaaHex =
      "0100000000010002d67a26c7fac4d4b223e92c218edd3329b14dad444dca3e909d53c38f52904a14c7e232f285d4efd8771984f4effd8e22c5694ca20486534dd0799d9c42807e016a7a575300000000000200000000000000000000000006803b132816e8f6d277ad7984305b2f590b038d0000000000000003ca9945ff10000000000000000000000000fd2322c8b9d79c605a4263f1bacec9a0b4ff6d42d03c78c99b240126e5e18bf07bae40847bb41b345b1850c71afb75e809f24d0800910000000000000000000000000000000000000000000000000000000000000000000000000000000000000000d10d649a5beb2cf5bf0c086c82ed1b1bbdf43bb0004f994e5454080000000005f5e10000000000000000000000000088c4f2412c1f2ca5d459e3355c44f04be7ba8ca8b57bfe41337237d696834be81e491c78099971fc2bc1ad6eedd032583350a516004b0000";
    const vaa = deserialize("Uint8Array", encoding.hex.decode(vaaHex));
    const digest = vaaDigest(vaa.hash);
    expect(digest).toBe(
      "13b76708a8612fadf4d3c347702e062f09e18874952663822cd988c29ff0e158"
    );
    expect(redeemCommandId(digest)).toBe(`ntt-private-redeem-${digest}`);
    expect(() => redeemCommandId("abc")).toThrow(/64 lowercase hex/);
  });
});
