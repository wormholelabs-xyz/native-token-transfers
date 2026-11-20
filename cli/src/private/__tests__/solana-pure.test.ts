import { describe, expect, test } from "bun:test";

import {
  SolanaConfirmationUnknownError,
  SolanaTransactionExpiredError,
  checkSolanaKeyFileMode,
  checkSolanaPeerIsRegistryManager,
  checkRegistryDecimalsMatchChain,
  sendSignedSolanaTransaction,
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
  statuses: (string | null | "throw")[];
  blockHeights: (number | "throw")[];
}): SolanaStatusRpc {
  let statusCallCount = 0;
  let heightCallCount = 0;
  return {
    async getSignatureConfirmationStatus() {
      const status =
        script.statuses[Math.min(statusCallCount, script.statuses.length - 1)];
      statusCallCount++;
      if (status === "throw") throw new Error("status read failed");
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
      if (height === "throw") throw new Error("height read failed");
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

describe("waitForSolanaConfirmation recheck after the height limit", () => {
  test.each([
    [
      "stale status, recheck finds it confirmed: confirmed",
      [null, "confirmed"],
      "resolves",
    ],
    ["recheck finds it processed: unknown", [null, "processed"], "unknown"],
    ["recheck finds nothing: expired", [null, null], "expired"],
  ] as const)("%s", async (_caseName, statuses, expectedOutcome) => {
    const waiting = waitForSolanaConfirmation(
      fakeStatusRpc({ statuses: [...statuses], blockHeights: [201] }),
      "sig",
      200,
      { pollIntervalMs: 1, wallClockCapMs: 10_000, sleep: noSleep }
    );
    if (expectedOutcome === "resolves")
      await expect(waiting).resolves.toBeUndefined();
    if (expectedOutcome === "unknown")
      await expect(waiting).rejects.toBeInstanceOf(
        SolanaConfirmationUnknownError
      );
    if (expectedOutcome === "expired")
      await expect(waiting).rejects.toBeInstanceOf(
        SolanaTransactionExpiredError
      );
  });
});

describe("waitForSolanaConfirmation read errors", () => {
  test.each([
    [
      "status read throws, then confirmed: confirmed",
      { statuses: ["throw", "confirmed"], blockHeights: [100] },
      "resolves",
    ],
    [
      "height read throws, then confirmed: confirmed",
      { statuses: [null, "confirmed"], blockHeights: ["throw", 100] },
      "resolves",
    ],
    [
      "status read always throws: unknown",
      { statuses: ["throw"], blockHeights: [100] },
      "unknown",
    ],
    [
      "height read always throws: unknown",
      { statuses: [null], blockHeights: ["throw"] },
      "unknown",
    ],
    [
      "recheck after the limit throws: unknown",
      { statuses: [null, "throw"], blockHeights: [201] },
      "unknown",
    ],
  ] as const)("%s", async (_caseName, script, expectedOutcome) => {
    const waiting = waitForSolanaConfirmation(
      fakeStatusRpc({
        statuses: [...script.statuses],
        blockHeights: [...script.blockHeights],
      }),
      "sig-777",
      200,
      { pollIntervalMs: 1, wallClockCapMs: 20, sleep: noSleep }
    );
    if (expectedOutcome === "resolves") {
      await expect(waiting).resolves.toBeUndefined();
      return;
    }
    await expect(waiting).rejects.toBeInstanceOf(
      SolanaConfirmationUnknownError
    );
    await waiting.catch((err: Error) => {
      expect(err.message).toContain("sig-777");
      expect(err.message).toContain("do not re-run; check the signature");
    });
  });
});

describe("sendSignedSolanaTransaction", () => {
  test("a send error after signing is unknown and names the signature", async () => {
    const sending = sendSignedSolanaTransaction(
      fakeStatusRpc({ statuses: [null], blockHeights: [100] }),
      "sig-123",
      200,
      async () => {
        throw new Error("socket hang up");
      },
      { pollIntervalMs: 1, wallClockCapMs: 10_000, sleep: noSleep }
    );
    await expect(sending).rejects.toBeInstanceOf(
      SolanaConfirmationUnknownError
    );
    await sending.catch((err: Error) =>
      expect(err.message).toContain("sig-123")
    );
  });
});

describe("checkSolanaKeyFileMode", () => {
  test.each([[0o600], [0o400]])("mode %o accepted", (mode) => {
    expect(() => checkSolanaKeyFileMode(mode)).not.toThrow();
  });
  test.each([[0o644], [0o640], [0o604]])("mode %o rejected", (mode) => {
    expect(() => checkSolanaKeyFileMode(mode)).toThrow(/chmod 600/);
  });
});

describe("checkRegistryDecimalsMatchChain", () => {
  test("equal passes, different rejects", () => {
    expect(() =>
      checkRegistryDecimalsMatchChain(9, 9, "Solana mint")
    ).not.toThrow();
    expect(() => checkRegistryDecimalsMatchChain(6, 9, "Solana mint")).toThrow(
      /decimals/
    );
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
