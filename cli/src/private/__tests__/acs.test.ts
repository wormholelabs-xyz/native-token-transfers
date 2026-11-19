import { describe, expect, test } from "bun:test";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import {
  collectEntries,
  findCoins,
  findCoreState,
  findDepositPreapproval,
  findEmitter,
  findLockedLedger,
  findNttManager,
  findReplayNode,
  selectCoin,
} from "../canton/acs.js";

const fixturePath = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "fixtures/acs-fixture.json"
);
const fixture = JSON.parse(fs.readFileSync(fixturePath, "utf8"));

const MGR_A =
  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const MGR_B =
  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
const ALICE = "alice::ns";
const GOVERNANCE = "guardianGovernance::ns";

describe("collectEntries", () => {
  test("walks the nested wrapper shape and finds every entry", () => {
    const entries = collectEntries(fixture);
    // 1 CoreState + 2 managers + 2 emitters + 1 ledger + 1 deposit
    // preapproval + 2 replay nodes + 5 coins = 14.
    expect(entries.length).toBe(1 + 2 + 2 + 1 + 1 + 2 + 5);
  });

  test("tolerates a bare array of createdEvent-shaped objects too", () => {
    const flat = [
      { templateId: "pkg:Mod:T", contractId: "c1", createArgument: { a: 1 } },
    ];
    expect(collectEntries(flat)).toEqual([
      { templateId: "pkg:Mod:T", contractId: "c1", createArgument: { a: 1 } },
    ]);
  });
});

describe("finders", () => {
  const entries = collectEntries(fixture);

  test("findCoreState resolves the single CoreState", () => {
    expect(findCoreState(entries)?.contractId).toBe("core-state-cid");
  });

  test("findNttManager distinguishes two managers by managerAddress", () => {
    expect(findNttManager(entries, MGR_A)?.contractId).toBe("manager-a-cid");
    expect(findNttManager(entries, MGR_B)?.contractId).toBe("manager-b-cid");
  });

  test("findNttManager returns undefined for an address not on the ledger", () => {
    expect(findNttManager(entries, "c".repeat(64))).toBeUndefined();
  });

  test("findEmitter matches by owner + emitterId", () => {
    expect(findEmitter(entries, GOVERNANCE, "em-a")?.contractId).toBe(
      "emitter-a-cid"
    );
    expect(findEmitter(entries, GOVERNANCE, "em-b")?.contractId).toBe(
      "emitter-b-cid"
    );
  });

  test("findLockedLedger matches by managerAddress", () => {
    const ledger = findLockedLedger(entries, MGR_A);
    expect(ledger?.contractId).toBe("locked-ledger-a-cid");
    expect(ledger?.createArgument.custodyHoldingCid).toBe("coin-custody-a");
    expect(findLockedLedger(entries, MGR_B)).toBeUndefined();
  });

  test("findDepositPreapproval matches by owner + instrumentId", () => {
    expect(
      findDepositPreapproval(entries, ALICE, "instrA")?.contractId
    ).toBe("deposit-preapproval-a-cid");
    expect(findDepositPreapproval(entries, ALICE, "instrB")).toBeUndefined();
  });

  test("findReplayNode matches by namespace", () => {
    expect(findReplayNode(entries, "ns-a")?.contractId).toBe(
      "replay-node-a-cid"
    );
    expect(findReplayNode(entries, "ns-b")?.contractId).toBe(
      "replay-node-b-cid"
    );
  });

  test("findCoins filters by owner + instrument, excluding other instruments", () => {
    const coins = findCoins(entries, ALICE, "instrA");
    expect(coins.map((c) => c.contractId).sort()).toEqual(
      ["coin-1", "coin-3", "coin-4", "coin-custody-a"].sort()
    );
  });
});

describe("selectCoin", () => {
  const entries = collectEntries(fixture);
  const coins = findCoins(entries, ALICE, "instrA");

  test("picks the first sufficient holding in ACS order, excluding custody", () => {
    // DAML Decimal strings: coin-1=10.0 (too small), coin-custody-a=500.0 (excluded), coin-3=50.0 (first hit).
    const picked = selectCoin(coins, 30_0000000000n, 10, "coin-custody-a");
    expect(picked.contractId).toBe("coin-3");
  });

  test("excludes the custody holding even when it would otherwise be first", () => {
    // coin-1=10, coin-3=50 (too small), coin-custody-a=500 (excluded), coin-4=100 (first hit).
    const picked = selectCoin(coins, 80_0000000000n, 10, "coin-custody-a");
    expect(picked.contractId).toBe("coin-4");
  });

  test("throws a descriptive error listing available holdings when none suffice", () => {
    expect(() => selectCoin(coins, 1000_0000000000n, 10, "coin-custody-a")).toThrow(
      /no single holding with amount >= 10000000000000/
    );
    try {
      selectCoin(coins, 1000_0000000000n, 10, "coin-custody-a");
      throw new Error("expected selectCoin to throw");
    } catch (err) {
      const msg = (err as Error).message;
      expect(msg).toContain("coin-1");
      expect(msg).toContain("coin-3");
      expect(msg).toContain("coin-4");
      expect(msg).not.toContain("coin-custody-a");
    }
  });
});
