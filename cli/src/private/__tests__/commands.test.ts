// Pure unit tests for the recipient-resolution helpers in commands.ts.
// Offline, no network — these never touch CantonClient/evm.ts/solana.ts.
import { describe, expect, test } from "bun:test";
import { loadConfig } from "../config.js";
import { resolveRedeemCantonRecipient, resolveSendRecipient } from "../commands.js";

const config = loadConfig();
const canton = config.canton;

const ALICE_BINDING = "b57bfe41337237d696834be81e491c78099971fc2bc1ad6eedd032583350a516";
const OPERATOR_BINDING = "efd963b986b976980dc83bfc06974761a4fb4498683abe194242da93222e4051";

describe("resolveSendRecipient: Canton destination", () => {
  test("defaults to the registry user party, bound", () => {
    const r = resolveSendRecipient("Canton", canton, {}, {});
    expect(r.recipient32Hex).toBe(ALICE_BINDING);
    expect(r.cantonParty).toBe(`alice::${canton.namespace}`);
    expect(r.dangerousRaw).toBe(false);
  });

  test("--recipient overrides the party hint", () => {
    const r = resolveSendRecipient("Canton", canton, { recipient: "operator" }, {});
    expect(r.recipient32Hex).toBe(OPERATOR_BINDING);
    expect(r.cantonParty).toBe(`operator::${canton.namespace}`);
  });

  test("--recipient-raw bypasses the binding entirely and is flagged dangerous", () => {
    const raw = "11".repeat(32);
    const r = resolveSendRecipient("Canton", canton, { recipientRaw: `0x${raw}` }, {});
    expect(r.recipient32Hex).toBe(raw);
    expect(r.dangerousRaw).toBe(true);
    expect(r.cantonParty).toBeUndefined();
  });

  test("--recipient-raw of the wrong length is rejected", () => {
    expect(() =>
      resolveSendRecipient("Canton", canton, { recipientRaw: "0x1234" }, {})
    ).toThrow(/32 bytes/);
  });
});

describe("resolveSendRecipient: Ethereum destination", () => {
  const ETH_ADDR = "0x06803b132816e8F6d277Ad7984305B2F590b038D";

  test("defaults to the derived wallet address, pad32'd", () => {
    const r = resolveSendRecipient("Ethereum", canton, {}, { ethereumAddress: ETH_ADDR });
    expect(r.recipient32Hex).toBe(
      "00000000000000000000000006803b132816e8f6d277ad7984305b2f590b038d"
    );
    expect(r.dangerousRaw).toBe(false);
  });

  test("--recipient overrides the default", () => {
    const other = "0xD10d649a5BeB2cf5Bf0C086c82Ed1B1BbdF43BB0";
    const r = resolveSendRecipient("Ethereum", canton, { recipient: other }, { ethereumAddress: ETH_ADDR });
    expect(r.recipient32Hex).toBe(
      "000000000000000000000000d10d649a5beb2cf5bf0c086c82ed1b1bbdf43bb0"
    );
  });

  test("throws when there is no default and no override", () => {
    expect(() => resolveSendRecipient("Ethereum", canton, {}, {})).toThrow(/ETH_PRIVATE_KEY/);
  });
});

describe("resolveSendRecipient: Solana destination", () => {
  const SOL_PUBKEY = "HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1Gciw";

  test("defaults to the derived payer pubkey, base58-decoded", () => {
    const r = resolveSendRecipient("Solana", canton, {}, { solanaPubkey: SOL_PUBKEY });
    expect(r.recipient32Hex).toBe(
      "fc12b0dc42b4d8b280cf13e2ee66fb6b2e6a206b5776b3c8ec17830bd35ba524"
    );
  });

  test("throws when there is no default and no override", () => {
    expect(() => resolveSendRecipient("Solana", canton, {}, {})).toThrow(/SOLANA_PRIVATE_KEY/);
  });
});

describe("resolveRedeemCantonRecipient", () => {
  test("defaults to the registry user party", () => {
    const r = resolveRedeemCantonRecipient(canton, undefined);
    expect(r.party).toBe(`alice::${canton.namespace}`);
    expect(r.binding).toBe(ALICE_BINDING);
  });

  test("respects an explicit party hint", () => {
    const r = resolveRedeemCantonRecipient(canton, "operator");
    expect(r.party).toBe(`operator::${canton.namespace}`);
    expect(r.binding).toBe(OPERATOR_BINDING);
  });
});
