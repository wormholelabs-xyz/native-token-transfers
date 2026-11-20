import { describe, expect, test } from "bun:test";

import { requireDeployment, validateRegistry } from "../config.js";
import {
  cantonRedeem,
  cantonSend,
  redeemCommandId,
  transferCommandId,
} from "../canton-ops.js";
import {
  DisclosureServiceNotFoundError,
  parseFlowResponse,
  type DisclosureServiceClient,
  type FlowName,
  type FlowResponse,
  type ManagerSummary,
} from "../canton/disclosure-client.js";
import type { CantonLedger, UserHolding } from "../canton/ledger-client.js";
import {
  parseCantonContractId,
  parseCantonPartyId,
  parseHex32,
  parseRawAmount,
  parseTransferNonceU32,
} from "../types.js";
import { parseTransferVaa } from "../vaa.js";
import { ETH_TRANSFER_VAA } from "./fixtures/vaas.js";
import {
  RETIRED_CANTON_NAMESPACE,
  TEST_CORE_PACKAGE_ID,
  TEST_NTT_PACKAGE_ID,
  retiredCanburnRegistryJson,
} from "./fixtures/registry.js";

function contractIdNumber(contractNumber: number): string {
  return `00${contractNumber.toString(16).padStart(64, "0")}ca10`;
}

const NTT_TEMPLATE = `${TEST_NTT_PACKAGE_ID}:Wormhole.Ntt.Manager:NttManager`;
const ROLE_TEMPLATES: Record<string, string> = {
  NttManager: NTT_TEMPLATE,
  "transceiver Emitter": `${TEST_CORE_PACKAGE_ID}:Wormhole.Core.State:Emitter`,
  CoreState: `${TEST_CORE_PACKAGE_ID}:Wormhole.Core.State:CoreState`,
  "covering ReplayNode": `${TEST_CORE_PACKAGE_ID}:Wormhole.Core.Replay:ReplayNode`,
  LockedLedger: `${TEST_NTT_PACKAGE_ID}:Wormhole.Ntt.Ledger:LockedLedger`,
  DepositPreapproval: `${TEST_NTT_PACKAGE_ID}:Wormhole.Ntt.Deposit:DepositPreapproval`,
  "committed canonical CoinFactory": `${"cd".repeat(32)}:Token.CIP0056.Coin:CoinFactory`,
};

function flowJson(
  flow: FlowName,
  roles: string[],
  missing: string[] = []
): unknown {
  return {
    flow,
    disclosures: roles.map((role, index) => ({
      role,
      templateId: ROLE_TEMPLATES[role],
      contractId: contractIdNumber(index + 1),
      createdEventBlob: `blob-${index + 1}`,
      synchronizerId: "global-domain::1220aa",
    })),
    missing: missing.map((role) => ({
      role,
      reason: "not visible to any reader party",
    })),
  };
}

function registryWithMode(mode: "burning" | "locking") {
  const registryJson = retiredCanburnRegistryJson();
  registryJson.deployments.canburn.chains.Canton.mode = mode;
  const registry = validateRegistry(registryJson);
  return { registry, deployment: requireDeployment(registry, "canburn") };
}

const USER = parseCantonPartyId(`alice::${RETIRED_CANTON_NAMESPACE}`);

function fakes(options: {
  mode: "burning" | "locking";
  flow: unknown;
  flowStatus?: 404;
  managerContractNumber?: number;
}) {
  const { registry, deployment } = registryWithMode(options.mode);
  const canton = deployment.chains.Canton!;
  const flowRequests: { flow: FlowName; parameters: Record<string, string> }[] =
    [];
  const submitted: any[] = [];
  const disclosure: DisclosureServiceClient = {
    async fetchFlow(flow, parameters) {
      flowRequests.push({ flow, parameters });
      if (options.flowStatus === 404)
        throw new DisclosureServiceNotFoundError("no DepositPreapproval found");
      return parseFlowResponse(options.flow, flow);
    },
    async fetchManagers(): Promise<ManagerSummary[]> {
      return [
        {
          managerAddress: canton.manager,
          contractId: parseCantonContractId(
            contractIdNumber(options.managerContractNumber ?? 1)
          ),
          tokenConfig: options.mode === "burning" ? "BurnMint" : "LockUnlock",
          instrumentId: canton.instrument,
          paused: false,
        },
      ];
    },
  };
  const holdings: UserHolding[] = [
    {
      contractId: parseCantonContractId(contractIdNumber(90)),
      amountDecimal: "5.0",
      instrumentId: canton.instrument,
      locked: false,
    },
    {
      contractId: parseCantonContractId(contractIdNumber(91)),
      amountDecimal: "50.0",
      instrumentId: {
        admin: parseCantonPartyId(`otherAdmin::${RETIRED_CANTON_NAMESPACE}`),
        id: canton.instrument.id,
      },
      locked: false,
    },
  ];
  const ledger: CantonLedger = {
    userPartyId: USER,
    async readUserHoldings() {
      return holdings;
    },
    async submitAndWaitForTransaction(request) {
      submitted.push(request);
      return {
        events: [
          {
            ExercisedEvent: {
              templateId: NTT_TEMPLATE,
              choice: (request as any).commands.commands[0].ExerciseCommand
                .choice,
              exerciseResult: { sequence: "9" },
            },
          },
        ],
      };
    },
  };
  return { registry, deployment, disclosure, ledger, flowRequests, submitted };
}

const SEND_ROLES = [
  "NttManager",
  "transceiver Emitter",
  "CoreState",
  "committed canonical CoinFactory",
];

function sendInput(setup: ReturnType<typeof fakes>) {
  return {
    registry: setup.registry,
    deployment: setup.deployment,
    destinationChain: "Ethereum" as const,
    rawAmount: parseRawAmount(10n ** 10n),
    recipient: parseHex32("22".repeat(32)),
    transferNonce: parseTransferNonceU32(7),
  };
}

describe("cantonSend", () => {
  test("body: actAs == [user], no readAs, disclosures in flow order, holding of the right admin", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("transfer", SEND_ROLES),
    });
    const result = await cantonSend(
      setup.ledger,
      setup.disclosure,
      sendInput(setup),
      { dryRun: false }
    );
    expect(result?.sequence).toBe("9");
    expect(setup.submitted.length).toBe(1);
    const commands = setup.submitted[0].commands;
    expect(commands.actAs).toEqual([USER]);
    expect("readAs" in commands).toBe(false);
    expect(
      commands.disclosedContracts.map((contract: any) => contract.contractId)
    ).toEqual(SEND_ROLES.map((_role, index) => contractIdNumber(index + 1)));
    const exercise = commands.commands[0].ExerciseCommand;
    expect(exercise.templateId).toBe(NTT_TEMPLATE);
    expect(exercise.contractId).toBe(contractIdNumber(1));
    expect(exercise.choiceArgument.ledgerCid).toBeNull();
    expect(exercise.choiceArgument.nonce).toBe("7");
    expect(exercise.choiceArgument.inputHoldingCids).toEqual([
      contractIdNumber(90),
    ]);
    expect(setup.flowRequests).toEqual([
      {
        flow: "transfer",
        parameters: { manager: setup.deployment.chains.Canton!.manager },
      },
    ]);
  });

  test.each([["NttManager"], ["transceiver Emitter"], ["CoreState"]])(
    "role %s missing: 0 submits",
    async (absentRole) => {
      const setup = fakes({
        mode: "burning",
        flow: flowJson(
          "transfer",
          SEND_ROLES.filter((role) => role !== absentRole)
        ),
      });
      await expect(
        cantonSend(setup.ledger, setup.disclosure, sendInput(setup), {
          dryRun: false,
        })
      ).rejects.toThrow(absentRole);
      expect(setup.submitted.length).toBe(0);
    }
  );

  test("locking mode without LockedLedger: 0 submits", async () => {
    const setup = fakes({
      mode: "locking",
      flow: flowJson("transfer", [
        "NttManager",
        "transceiver Emitter",
        "CoreState",
      ]),
    });
    await expect(
      cantonSend(setup.ledger, setup.disclosure, sendInput(setup), {
        dryRun: false,
      })
    ).rejects.toThrow(/LockedLedger/);
    expect(setup.submitted.length).toBe(0);
  });

  test("missing not empty: throw naming the role, 0 submits", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("transfer", SEND_ROLES, ["committed TransferFactory"]),
    });
    await expect(
      cantonSend(setup.ledger, setup.disclosure, sendInput(setup), {
        dryRun: false,
      })
    ).rejects.toThrow(/committed TransferFactory/);
    expect(setup.submitted.length).toBe(0);
  });

  test("NttManager role differs from /v1/managers: 0 submits", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("transfer", SEND_ROLES),
      managerContractNumber: 77,
    });
    await expect(
      cantonSend(setup.ledger, setup.disclosure, sendInput(setup), {
        dryRun: false,
      })
    ).rejects.toThrow(/NttManager/);
    expect(setup.submitted.length).toBe(0);
  });

  test("dry run: 0 submits", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("transfer", SEND_ROLES),
    });
    expect(
      await cantonSend(setup.ledger, setup.disclosure, sendInput(setup), {
        dryRun: true,
      })
    ).toBeUndefined();
    expect(setup.submitted.length).toBe(0);
  });
});

describe("parseFlowResponse", () => {
  test.each([
    [
      "flow name differs",
      { ...(flowJson("mint", SEND_ROLES) as object) },
      /flow/,
    ],
    [
      "duplicate role",
      flowJson("transfer", ["NttManager", "NttManager"]),
      /more than once/,
    ],
    [
      "empty blob",
      {
        flow: "transfer",
        disclosures: [
          {
            role: "CoreState",
            templateId: ROLE_TEMPLATES.CoreState,
            contractId: contractIdNumber(1),
            createdEventBlob: "",
            synchronizerId: "s",
          },
        ],
        missing: [],
      },
      /createdEventBlob/,
    ],
    ["not an object", [], /object/],
  ])("rejects %s", (_caseName, responseJson, expectedError) => {
    expect(() => parseFlowResponse(responseJson, "transfer")).toThrow(
      expectedError
    );
  });
});

const MINT_ROLES = [
  "NttManager",
  "CoreState",
  "covering ReplayNode",
  "DepositPreapproval",
  "committed canonical CoinFactory",
];

describe("cantonRedeem", () => {
  const parsedVaa = parseTransferVaa(ETH_TRANSFER_VAA);

  test("mint: replayNodeCid from the covering ReplayNode role, digest in the request, id from the digest", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("mint", MINT_ROLES),
    });
    await cantonRedeem(
      setup.ledger,
      setup.disclosure,
      {
        registry: setup.registry,
        deployment: setup.deployment,
        parsedVaa,
        recipient: USER,
      },
      { dryRun: false }
    );
    expect(setup.flowRequests[0]).toEqual({
      flow: "mint",
      parameters: {
        manager: setup.deployment.chains.Canton!.manager,
        digest: parsedVaa.digest,
        recipient: USER,
      },
    });
    const commands = setup.submitted[0].commands;
    expect(commands.commandId).toBe(redeemCommandId(parsedVaa.digest));
    const argument = commands.commands[0].ExerciseCommand.choiceArgument;
    expect(argument.replayNodeCid).toBe(contractIdNumber(3));
    expect(argument.executor).toBe(USER);
    expect(argument.pubKeys.length).toBe(1);
    expect(argument.pubKeys[0]._1).toBe("0");
  });

  test("flow without the covering ReplayNode: 0 submits", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson(
        "mint",
        MINT_ROLES.filter((role) => role !== "covering ReplayNode")
      ),
    });
    await expect(
      cantonRedeem(
        setup.ledger,
        setup.disclosure,
        {
          registry: setup.registry,
          deployment: setup.deployment,
          parsedVaa,
          recipient: USER,
        },
        { dryRun: false }
      )
    ).rejects.toThrow(/covering ReplayNode/);
    expect(setup.submitted.length).toBe(0);
  });

  test("404 for mint: the VAA stays redeemable", async () => {
    const setup = fakes({ mode: "burning", flow: undefined, flowStatus: 404 });
    await expect(
      cantonRedeem(
        setup.ledger,
        setup.disclosure,
        {
          registry: setup.registry,
          deployment: setup.deployment,
          parsedVaa,
          recipient: USER,
        },
        { dryRun: false }
      )
    ).rejects.toThrow(/nothing has been submitted, the VAA stays redeemable/);
    expect(setup.submitted.length).toBe(0);
  });

  test("retry after contract not active stops at 3 attempts with one command id", async () => {
    const setup = fakes({
      mode: "burning",
      flow: flowJson("mint", MINT_ROLES),
    });
    setup.ledger.submitAndWaitForTransaction = async (request) => {
      setup.submitted.push(request);
      throw new Error("LOCAL_VERDICT_INACTIVE_CONTRACTS: contract not active");
    };
    await expect(
      cantonRedeem(
        setup.ledger,
        setup.disclosure,
        {
          registry: setup.registry,
          deployment: setup.deployment,
          parsedVaa,
          recipient: USER,
        },
        { dryRun: false }
      )
    ).rejects.toThrow(/INACTIVE/);
    expect(setup.submitted.length).toBe(3);
    expect(
      new Set(setup.submitted.map((request) => request.commands.commandId)).size
    ).toBe(1);
    expect(setup.flowRequests.length).toBe(3);
  });
});

describe("command ids", () => {
  const { registry, deployment } = registryWithMode("burning");
  const base = {
    user: USER,
    manager: deployment.chains.Canton!.manager,
    rawAmount: parseRawAmount(5n),
    recipientChainId: registry.chainIds.Ethereum,
    recipient: parseHex32("22".repeat(32)),
    transferNonce: parseTransferNonceU32(1),
  };

  test("same inputs give the same id; each changed input gives a different id", () => {
    const baseId = transferCommandId(base);
    expect(transferCommandId({ ...base })).toBe(baseId);
    expect(
      transferCommandId({ ...base, rawAmount: parseRawAmount(6n) })
    ).not.toBe(baseId);
    expect(
      transferCommandId({ ...base, transferNonce: parseTransferNonceU32(2) })
    ).not.toBe(baseId);
    expect(
      transferCommandId({ ...base, recipient: parseHex32("23".repeat(32)) })
    ).not.toBe(baseId);
  });

  test("redeem id is the digest form", () => {
    const digest = parseTransferVaa(ETH_TRANSFER_VAA).digest;
    expect(redeemCommandId(digest)).toBe(`ntt-private-redeem-${digest}`);
  });
});
