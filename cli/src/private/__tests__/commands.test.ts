import { describe, expect, test } from "bun:test";

import { validateRegistry } from "../config.js";
import {
  requireTypedConfirmation,
  type SubmitConfirmation,
} from "../confirmation.js";
import { resolveSendRecipient } from "../recipient.js";
import { parseHex32 } from "../types.js";
import {
  RETIRED_CANTON_NAMESPACE,
  retiredCanburnRegistryJson,
} from "./fixtures/registry.js";

const registry = validateRegistry(retiredCanburnRegistryJson());
const ALICE = `alice::${RETIRED_CANTON_NAMESPACE}`;
const ALICE_BINDING =
  "642e20b7253475adf8c136f61e1258a70c3809ec07256f5adef300f181923413";

describe("resolveSendRecipient", () => {
  test("Canton default: the registry user party, bound", () => {
    const resolved = resolveSendRecipient("Canton", registry, {}, {});
    expect(resolved.recipient as string).toBe(ALICE_BINDING);
    expect(resolved.cantonPartyId as string | undefined).toBe(ALICE);
  });

  test("Canton: a complete party id is used unchanged", () => {
    expect(
      resolveSendRecipient("Canton", registry, { recipient: ALICE }, {})
        .recipient as string
    ).toBe(ALICE_BINDING);
  });

  test.each([
    ["bob"],
    [`bob::${RETIRED_CANTON_NAMESPACE}::${RETIRED_CANTON_NAMESPACE}`],
  ])("Canton: %s rejects and returns no binding", (recipientText) => {
    expect(() =>
      resolveSendRecipient("Canton", registry, { recipient: recipientText }, {})
    ).toThrow(/party/);
  });

  test("Ethereum: checksummed address", () => {
    const resolved = resolveSendRecipient(
      "Ethereum",
      registry,
      { recipient: "0x06803b132816e8F6d277Ad7984305B2F590b038D" },
      {}
    );
    expect(resolved.recipient as string).toBe(
      "00000000000000000000000006803b132816e8f6d277ad7984305b2f590b038d"
    );
  });

  test("Ethereum: lowercase without the flag rejects", () => {
    expect(() =>
      resolveSendRecipient(
        "Ethereum",
        registry,
        { recipient: "0x06803b132816e8f6d277ad7984305b2f590b038d" },
        {}
      )
    ).toThrow(/checksum/);
  });

  test("Solana: no recipient names --recipient only", () => {
    let message = "";
    try {
      resolveSendRecipient("Solana", registry, {}, {});
    } catch (err) {
      message = (err as Error).message;
    }
    expect(message).toMatch(/--recipient/);
    expect(message).not.toMatch(/SOLANA_PRIVATE_KEY/);
  });

  test("Solana: 2 characters lost rejects", () => {
    expect(() =>
      resolveSendRecipient(
        "Solana",
        registry,
        { recipient: "HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1Gc" },
        {}
      )
    ).toThrow(/32 bytes/);
  });

  test.each([["Canton"], ["Ethereum"], ["Solana"]] as const)(
    "--recipient-raw without --i-accept-raw-recipient rejects for %s",
    (destinationChain) => {
      expect(() =>
        resolveSendRecipient(
          destinationChain,
          registry,
          { recipientRaw: "11".repeat(32) },
          {}
        )
      ).toThrow(/i-accept-raw-recipient/);
    }
  );

  test("--recipient-raw with the flag is used and marked raw", () => {
    const resolved = resolveSendRecipient(
      "Ethereum",
      registry,
      { recipientRaw: `0x${"11".repeat(32)}`, acceptRawRecipient: true },
      {}
    );
    expect(resolved.isRaw).toBe(true);
    expect(resolved.recipient as string).toBe("11".repeat(32));
  });
});

const confirmation: SubmitConfirmation = {
  network: "Mainnet",
  operation: "send",
  deploymentName: "canburn",
  route: "Ethereum -> Canton",
  amountText: "1.5",
  recipientDescription: "test",
  recipient: parseHex32("ab".repeat(28) + "12345678"),
};

function scriptedPrompt(answers: string[]) {
  const questions: string[] = [];
  return {
    questions,
    prompt: async (question: string) => {
      questions.push(question);
      return answers.shift() ?? "";
    },
  };
}

describe("requireTypedConfirmation", () => {
  test.each([
    ["no --yes-recipient", undefined, /--yes-recipient/],
    [
      "a different party",
      `bob::${RETIRED_CANTON_NAMESPACE}`,
      /--yes-recipient/,
    ],
  ])(
    "--yes with a Canton recipient and %s: refused",
    async (_caseName, confirmedCantonRecipient, expectedError) => {
      const scripted = scriptedPrompt([]);
      await expect(
        requireTypedConfirmation(
          { ...confirmation, cantonRecipientPartyId: ALICE as never },
          {
            assumeYes: true,
            stdinIsTty: false,
            prompt: scripted.prompt,
            confirmedCantonRecipient,
          }
        )
      ).rejects.toThrow(expectedError);
    }
  );

  test("--yes with --yes-recipient equal to the Canton recipient: no prompt", async () => {
    const scripted = scriptedPrompt([]);
    await requireTypedConfirmation(
      { ...confirmation, cantonRecipientPartyId: ALICE as never },
      {
        assumeYes: true,
        stdinIsTty: false,
        prompt: scripted.prompt,
        confirmedCantonRecipient: ALICE,
      }
    );
    expect(scripted.questions.length).toBe(0);
  });

  test("correct network and suffix: confirmed", async () => {
    const scripted = scriptedPrompt(["Mainnet", "12345678"]);
    await expect(
      requireTypedConfirmation(confirmation, {
        assumeYes: false,
        stdinIsTty: true,
        prompt: scripted.prompt,
      })
    ).resolves.toBeUndefined();
  });

  test.each([
    ["wrong network", ["Testnet", "12345678"]],
    ["wrong suffix", ["Mainnet", "12345679"]],
  ])("%s: refused", async (_caseName, answers) => {
    const scripted = scriptedPrompt(answers);
    await expect(
      requireTypedConfirmation(confirmation, {
        assumeYes: false,
        stdinIsTty: true,
        prompt: scripted.prompt,
      })
    ).rejects.toThrow(/nothing has been submitted/);
  });

  test("Canton recipient: the full party id must be typed", async () => {
    const cantonConfirmation = {
      ...confirmation,
      cantonRecipientPartyId: ALICE as never,
    };
    const wrong = scriptedPrompt(["Mainnet", "alice"]);
    await expect(
      requireTypedConfirmation(cantonConfirmation, {
        assumeYes: false,
        stdinIsTty: true,
        prompt: wrong.prompt,
      })
    ).rejects.toThrow();
    const right = scriptedPrompt(["Mainnet", ALICE]);
    await expect(
      requireTypedConfirmation(cantonConfirmation, {
        assumeYes: false,
        stdinIsTty: true,
        prompt: right.prompt,
      })
    ).resolves.toBeUndefined();
  });

  test("--yes: no prompt", async () => {
    const scripted = scriptedPrompt([]);
    await requireTypedConfirmation(confirmation, {
      assumeYes: true,
      stdinIsTty: false,
      prompt: scripted.prompt,
    });
    expect(scripted.questions.length).toBe(0);
  });

  test("not a TTY and no --yes: refused", async () => {
    const scripted = scriptedPrompt(["Mainnet", "12345678"]);
    await expect(
      requireTypedConfirmation(confirmation, {
        assumeYes: false,
        stdinIsTty: false,
        prompt: scripted.prompt,
      })
    ).rejects.toThrow(/--yes/);
    expect(scripted.questions.length).toBe(0);
  });
});
