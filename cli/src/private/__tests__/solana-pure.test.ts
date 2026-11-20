import { describe, expect, test } from "bun:test";

import {
  SolanaConfirmationUnknownError,
  SolanaTransactionExpiredError,
  checkSolanaPeerIsRegistryManager,
  parseSolanaKeypairFileText,
  runSolanaRedeemSteps,
  waitForSolanaConfirmation,
  type SolanaStatusRpc,
} from "../solana-pure.js";
import { parseHex32 } from "../types.js";

describe("parseSolanaKeypairFileText", () => {
  const validKeypairText = JSON.stringify(
    Array.from({ length: 64 }, (_unused, index) => index)
  );

  test("accepts 64 byte values", () => {
    expect(parseSolanaKeypairFileText(validKeypairText).length).toBe(64);
  });

  test.each([
    ["bad JSON", "{secret-material"],
    ["63 values", JSON.stringify(Array(63).fill(1))],
    ["65 values", JSON.stringify(Array(65).fill(1))],
    ["value 256", JSON.stringify([...Array(63).fill(1), 256])],
    ["value -1", JSON.stringify([...Array(63).fill(1), -1])],
    ["an object", JSON.stringify({ secretKey: [] })],
  ])("rejects %s without echoing the file content", (_caseName, fileText) => {
    let message = "";
    try {
      parseSolanaKeypairFileText(fileText);
    } catch (err) {
      message = (err as Error).message;
    }
    expect(message).toMatch(/SOLANA_PAYER_PATH/);
    expect(message).not.toContain("secret-material");
  });
});

function fakeStatusRpc(script: {
  statuses: (string | null)[];
  blockHeights: number[];
}): SolanaStatusRpc {
  let statusCallCount = 0;
  let heightCallCount = 0;
  return {
    async getSignatureConfirmationStatus() {
      const status =
        script.statuses[Math.min(statusCallCount, script.statuses.length - 1)];
      statusCallCount++;
      return status === null
        ? undefined
        : { confirmationStatus: status, failed: false };
    },
    async getBlockHeight() {
      const height =
        script.blockHeights[
          Math.min(heightCallCount, script.blockHeights.length - 1)
        ];
      heightCallCount++;
      return height;
    },
  };
}

const noSleep = async () => {};

describe("waitForSolanaConfirmation", () => {
  test("confirms", async () => {
    await expect(
      waitForSolanaConfirmation(
        fakeStatusRpc({ statuses: [null, "confirmed"], blockHeights: [100] }),
        "sig",
        200,
        {
          pollIntervalMs: 1,
          wallClockCapMs: 10_000,
          sleep: noSleep,
        }
      )
    ).resolves.toBeUndefined();
  });

  test("block height passes the limit: expired, safe to retry", async () => {
    await expect(
      waitForSolanaConfirmation(
        fakeStatusRpc({ statuses: [null], blockHeights: [100, 201] }),
        "sig",
        200,
        {
          pollIntervalMs: 1,
          wallClockCapMs: 10_000,
          sleep: noSleep,
        }
      )
    ).rejects.toBeInstanceOf(SolanaTransactionExpiredError);
  });

  test("wall-clock cap first: unknown", async () => {
    await expect(
      waitForSolanaConfirmation(
        fakeStatusRpc({ statuses: [null], blockHeights: [100] }),
        "sig",
        200,
        {
          pollIntervalMs: 1,
          wallClockCapMs: 0,
          sleep: noSleep,
        }
      )
    ).rejects.toBeInstanceOf(SolanaConfirmationUnknownError);
  });
});

describe("checkSolanaPeerIsRegistryManager", () => {
  const expected = parseHex32("ab".repeat(32));
  test("equal passes", () => {
    expect(() =>
      checkSolanaPeerIsRegistryManager(expected, expected, "Canton")
    ).not.toThrow();
  });
  test("1 byte different rejects", () => {
    expect(() =>
      checkSolanaPeerIsRegistryManager(
        parseHex32("ac" + "ab".repeat(31)),
        expected,
        "Canton"
      )
    ).toThrow(/peer/);
  });
});

describe("runSolanaRedeemSteps", () => {
  test("dry run with a missing token account throws before any transaction", async () => {
    let driveCount = 0;
    await expect(
      runSolanaRedeemSteps(
        {
          checkRecipientTokenAccount: async () => {
            throw new Error("associated token account does not exist");
          },
          driveRedeem: async () => {
            driveCount++;
            return "sig";
          },
        },
        { dryRun: true }
      )
    ).rejects.toThrow(/token account/);
    expect(driveCount).toBe(0);
  });
});
