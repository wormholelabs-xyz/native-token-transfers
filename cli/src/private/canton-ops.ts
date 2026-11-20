// Canton send and redeem: disclosure-service flow -> registry checks -> one submit as the user.
import { AbiCoder, keccak256 } from "ethers";

import { bytesToHex } from "./binding.js";
import type {
  CantonChainConfig,
  CantonInstrumentId,
  ChainName,
  Deployment,
  PrivateRegistry,
} from "./config.js";
import {
  DisclosureServiceNotFoundError,
  requireFlowRole,
  type DisclosureServiceClient,
  type FlowName,
  type FlowResponse,
} from "./canton/disclosure-client.js";
import { recoverGuardianPublicKeys } from "./canton/guardian-keys.js";
import type {
  CantonLedger,
  LedgerTransaction,
  UserHolding,
} from "./canton/ledger-client.js";
import {
  templateModuleAndEntity,
  templatePackageId,
  type CantonContractId,
  type CantonPartyId,
  type Hex32,
  type RawAmount,
  type TransferNonceU32,
  type VaaDigestHex32,
  type WormholeChainId,
} from "./types.js";
import type { ParsedTransferVaa } from "./vaa.js";

export const REDEEM_SUBMIT_MAX_ATTEMPTS = 3;
export const INPUT_HOLDINGS_MAX = 16;
// Instant finality on Canton; the core watcher accepts 0.
const TRANSFER_CONSISTENCY_LEVEL = 0;

const ROLE_NTT_MANAGER = "NttManager";
const ROLE_CORE_STATE = "CoreState";
const ROLE_TRANSCEIVER_EMITTER = "transceiver Emitter";
const ROLE_COVERING_REPLAY_NODE = "covering ReplayNode";
const ROLE_LOCKED_LEDGER = "LockedLedger";
const ROLE_DEPOSIT_PREAPPROVAL = "DepositPreapproval";

const NTT_PACKAGE_ROLES = [
  ROLE_NTT_MANAGER,
  ROLE_LOCKED_LEDGER,
  ROLE_DEPOSIT_PREAPPROVAL,
];
const CORE_PACKAGE_ROLES = [
  ROLE_CORE_STATE,
  ROLE_TRANSCEIVER_EMITTER,
  ROLE_COVERING_REPLAY_NODE,
];
const NTT_MANAGER_MODULE_AND_ENTITY = "Wormhole.Ntt.Manager:NttManager";
const EMPTY_EXTRA_ARGS = { context: { values: {} }, meta: { values: {} } };
const INACTIVE_CONTRACT_PATTERN =
  /INACTIVE_CONTRACTS|CONTRACT_NOT_FOUND|contract not active/i;

type TransferLedgerArgument =
  | { tokenMode: "burning" }
  | { tokenMode: "locking"; lockedLedgerCid: CantonContractId };

export function transferCommandId(fields: {
  user: CantonPartyId;
  manager: Hex32;
  rawAmount: RawAmount;
  recipientChainId: WormholeChainId;
  recipient: Hex32;
  transferNonce: TransferNonceU32;
}): string {
  const encodedFields = AbiCoder.defaultAbiCoder().encode(
    ["string", "bytes32", "uint256", "uint16", "bytes32", "uint32"],
    [
      fields.user,
      `0x${fields.manager}`,
      fields.rawAmount,
      fields.recipientChainId,
      `0x${fields.recipient}`,
      fields.transferNonce,
    ]
  );
  return `ntt-private-send-${keccak256(encodedFields).slice(2)}`;
}

export function redeemCommandId(digest: VaaDigestHex32): string {
  return `ntt-private-redeem-${digest}`;
}

function cantonLeg(deployment: Deployment): CantonChainConfig {
  const canton = deployment.chains.Canton;
  if (!canton)
    throw new Error(`deployment "${deployment.name}" has no Canton leg`);
  return canton;
}

function sameInstrument(
  left: CantonInstrumentId,
  right: CantonInstrumentId
): boolean {
  return left.admin === right.admin && left.id === right.id;
}

/** Compares a flow with the registry and with /v1/managers before any submit. */
async function checkFlowMatchesRegistry(
  flow: FlowResponse,
  canton: CantonChainConfig,
  disclosure: DisclosureServiceClient
): Promise<void> {
  if (flow.missingRoles.length > 0) {
    throw new Error(
      `disclosure service flow "${flow.flow}" cannot supply: ${flow.missingRoles.join(", ")}; nothing has been submitted`
    );
  }
  const managerRole = requireFlowRole(flow, ROLE_NTT_MANAGER);
  if (
    templateModuleAndEntity(managerRole.templateId) !==
    NTT_MANAGER_MODULE_AND_ENTITY
  ) {
    throw new Error(`NttManager role has template ${managerRole.templateId}`);
  }
  for (const disclosed of flow.disclosures) {
    const packageId = templatePackageId(disclosed.templateId);
    if (
      NTT_PACKAGE_ROLES.includes(disclosed.role) &&
      packageId !== canton.vettedPackageIds.ntt
    ) {
      throw new Error(
        `role ${disclosed.role} has package ${packageId}; the registry vetted ntt package is ${canton.vettedPackageIds.ntt}`
      );
    }
    if (
      CORE_PACKAGE_ROLES.includes(disclosed.role) &&
      packageId !== canton.vettedPackageIds.core
    ) {
      throw new Error(
        `role ${disclosed.role} has package ${packageId}; the registry vetted core package is ${canton.vettedPackageIds.core}`
      );
    }
  }
  const managers = (await disclosure.fetchManagers()).filter(
    (manager) => manager.managerAddress === canton.manager
  );
  if (managers.length !== 1)
    throw new Error(
      `disclosure service lists ${managers.length} managers with address ${canton.manager}`
    );
  const [manager] = managers;
  if (manager.contractId !== managerRole.contractId) {
    throw new Error(
      "the NttManager role contract id differs from the /v1/managers entry for the registry manager"
    );
  }
  if (
    manager.tokenConfig !==
    (canton.mode === "locking" ? "LockUnlock" : "BurnMint")
  ) {
    throw new Error(
      `NttManager tokenConfig ${manager.tokenConfig} does not match registry mode ${canton.mode}`
    );
  }
  if (!sameInstrument(manager.instrumentId, canton.instrument)) {
    throw new Error(
      "NttManager instrument differs from the registry instrument"
    );
  }
  if (manager.reserveCustodian !== canton.reserveCustodian) {
    throw new Error(
      `NttManager reserveCustodian ${manager.reserveCustodian} differs from the registry ${canton.reserveCustodian}`
    );
  }
  if (manager.paused) throw new Error("the NttManager is paused");
}

/** Daml Decimal text -> raw units; rejects a non-zero digit past the token decimals. */
function holdingAmountRaw(amountDecimal: string, decimals: number): bigint {
  const match = /^(\d+)(?:\.(\d+))?$/.exec(amountDecimal);
  if (!match)
    throw new Error(`holding amount "${amountDecimal}" is not a Decimal`);
  const fractionDigits = match[2] ?? "";
  if (/[^0]/.test(fractionDigits.slice(decimals)))
    throw new Error(
      `holding amount "${amountDecimal}" exceeds ${decimals} decimals`
    );
  return BigInt(
    match[1] + fractionDigits.slice(0, decimals).padEnd(decimals, "0")
  );
}

/** Unlocked holdings of the registry instrument, by contract id order, until the sum covers the amount. */
export function selectInputHoldings(
  holdings: UserHolding[],
  canton: CantonChainConfig,
  rawAmount: RawAmount
): CantonContractId[] {
  const candidates = holdings
    .filter(
      (holding) =>
        !holding.locked &&
        sameInstrument(holding.instrumentId, canton.instrument)
    )
    .sort((left, right) =>
      left.contractId < right.contractId
        ? -1
        : left.contractId > right.contractId
          ? 1
          : 0
    );
  const selected: CantonContractId[] = [];
  let selectedRaw = 0n;
  for (const holding of candidates) {
    if (selectedRaw >= rawAmount) break;
    if (selected.length === INPUT_HOLDINGS_MAX) break;
    selected.push(holding.contractId);
    selectedRaw += holdingAmountRaw(holding.amountDecimal, canton.decimals);
  }
  if (selectedRaw < rawAmount) {
    throw new Error(
      `unlocked holdings of ${canton.instrument.id} (admin ${canton.instrument.admin}) cover ${selectedRaw} raw in ${INPUT_HOLDINGS_MAX} holdings or fewer; ${rawAmount} raw needed`
    );
  }
  return selected;
}

function buildSubmitRequest(
  user: CantonPartyId,
  commandId: string,
  exerciseCommand: Record<string, unknown>,
  flow: FlowResponse
): unknown {
  return {
    commands: {
      commands: [{ ExerciseCommand: exerciseCommand }],
      commandId,
      actAs: [user],
      disclosedContracts: flow.disclosures.map((disclosed) => ({
        templateId: disclosed.templateId,
        contractId: disclosed.contractId,
        createdEventBlob: disclosed.createdEventBlob,
        synchronizerId: disclosed.synchronizerId,
      })),
      synchronizerId: flow.disclosures[0].synchronizerId,
    },
    transactionFormat: {
      eventFormat: {
        filtersByParty: {
          [user]: {
            cumulative: [
              {
                identifierFilter: {
                  WildcardFilter: { value: { includeCreatedEventBlob: false } },
                },
              },
            ],
          },
        },
        verbose: true,
      },
      transactionShape: "TRANSACTION_SHAPE_LEDGER_EFFECTS",
    },
  };
}

export function extractTransferSequence(
  transaction: LedgerTransaction
): string {
  const transferResults = transaction.events
    .map((event: any) => event?.ExercisedEvent)
    .filter(
      (exercised: any) =>
        exercised?.choice === "Transfer" &&
        typeof exercised.templateId === "string" &&
        templateModuleAndEntity(exercised.templateId) ===
          NTT_MANAGER_MODULE_AND_ENTITY
    );
  if (transferResults.length !== 1)
    throw new Error(
      `expected one NttManager Transfer event, got ${transferResults.length}`
    );
  const sequence = transferResults[0].exerciseResult?.sequence;
  if (typeof sequence !== "string" || !/^[0-9]{1,20}$/.test(sequence))
    throw new Error("Transfer result has no sequence");
  return sequence;
}

export interface CantonSendInput {
  registry: PrivateRegistry;
  deployment: Deployment;
  destinationChain: ChainName;
  rawAmount: RawAmount;
  recipient: Hex32;
  transferNonce: TransferNonceU32;
}

export async function cantonSend(
  ledger: CantonLedger,
  disclosure: DisclosureServiceClient,
  input: CantonSendInput,
  options: { dryRun: boolean }
): Promise<{ sequence?: string; commandId: string } | undefined> {
  const canton = cantonLeg(input.deployment);
  const flow = await disclosure.fetchFlow("transfer", {
    manager: canton.manager,
  });
  const managerRole = requireFlowRole(flow, ROLE_NTT_MANAGER);
  const transceiverEmitterRole = requireFlowRole(
    flow,
    ROLE_TRANSCEIVER_EMITTER
  );
  const coreStateRole = requireFlowRole(flow, ROLE_CORE_STATE);
  const ledgerArgument: TransferLedgerArgument =
    canton.mode === "locking"
      ? {
          tokenMode: "locking",
          lockedLedgerCid: requireFlowRole(flow, ROLE_LOCKED_LEDGER).contractId,
        }
      : { tokenMode: "burning" };
  await checkFlowMatchesRegistry(flow, canton, disclosure);

  const inputHoldingCids = selectInputHoldings(
    await ledger.readUserHoldings(),
    canton,
    input.rawAmount
  );
  const recipientChainId = input.registry.chainIds[input.destinationChain];
  const commandId = transferCommandId({
    user: ledger.userPartyId,
    manager: canton.manager,
    rawAmount: input.rawAmount,
    recipientChainId,
    recipient: input.recipient,
    transferNonce: input.transferNonce,
  });
  const request = buildSubmitRequest(
    ledger.userPartyId,
    commandId,
    {
      templateId: managerRole.templateId,
      contractId: managerRole.contractId,
      choice: "Transfer",
      choiceArgument: {
        user: ledger.userPartyId,
        transceiverEmitterCid: transceiverEmitterRole.contractId,
        coreStateCid: coreStateRole.contractId,
        ledgerCid:
          ledgerArgument.tokenMode === "locking"
            ? ledgerArgument.lockedLedgerCid
            : null,
        recipientChain: String(recipientChainId),
        recipientAddress: input.recipient,
        rawAmount: input.rawAmount.toString(),
        nonce: String(input.transferNonce),
        consistencyLevel: String(TRANSFER_CONSISTENCY_LEVEL),
        inputHoldingCids,
        feeAllocation: null,
        extraArgs: EMPTY_EXTRA_ARGS,
      },
    },
    flow
  );
  if (options.dryRun) {
    console.log(
      `  [dry-run] would submit command ${commandId} with ${flow.disclosures.length} disclosed contracts`
    );
    return undefined;
  }
  let transaction: LedgerTransaction;
  try {
    transaction = await ledger.submitAndWaitForTransaction(request);
  } catch (err) {
    throw new Error(
      `Canton submit of command ${commandId}: result unknown (${(err as Error).message}). ` +
        `Run the same send with --transfer-nonce ${input.transferNonce}: the command id is then the same, and the participant rejects a duplicate.`
    );
  }
  try {
    return { sequence: extractTransferSequence(transaction), commandId };
  } catch (err) {
    console.log(
      `  command ${commandId} committed; the sequence is not readable (${(err as Error).message})`
    );
    return { commandId };
  }
}

export interface CantonRedeemInput {
  registry: PrivateRegistry;
  deployment: Deployment;
  parsedVaa: ParsedTransferVaa;
  recipient: CantonPartyId;
}

async function fetchRedeemFlow(
  disclosure: DisclosureServiceClient,
  flowName: FlowName,
  parameters: Record<string, string>
) {
  try {
    return await disclosure.fetchFlow(flowName, parameters);
  } catch (err) {
    if (err instanceof DisclosureServiceNotFoundError) {
      throw new Error(
        `${err.message}; nothing has been submitted, the VAA stays redeemable`
      );
    }
    throw err;
  }
}

function buildRedeemRequest(
  ledger: CantonLedger,
  canton: CantonChainConfig,
  input: CantonRedeemInput,
  flow: FlowResponse,
  commandId: string
) {
  const managerRole = requireFlowRole(flow, ROLE_NTT_MANAGER);
  const sharedArgument = {
    executor: ledger.userPartyId,
    coreStateCid: requireFlowRole(flow, ROLE_CORE_STATE).contractId,
    replayNodeCid: requireFlowRole(flow, ROLE_COVERING_REPLAY_NODE).contractId,
    vaaBytes: bytesToHex(input.parsedVaa.bytes),
    pubKeys: recoverGuardianPublicKeys(input.parsedVaa).map((guardianKey) => ({
      _1: String(guardianKey.guardianIndex),
      _2: guardianKey.uncompressedPublicKeyHex,
    })),
    recipient: input.recipient,
  };
  const exerciseCommand =
    canton.mode === "locking"
      ? {
          templateId: managerRole.templateId,
          contractId: managerRole.contractId,
          choice: "Release",
          choiceArgument: {
            ...sharedArgument,
            ledgerCid: requireFlowRole(flow, ROLE_LOCKED_LEDGER).contractId,
            inputHoldingCids: [],
            extraArgs: EMPTY_EXTRA_ARGS,
          },
        }
      : {
          templateId: managerRole.templateId,
          contractId: managerRole.contractId,
          choice: "Mint",
          choiceArgument: {
            ...sharedArgument,
            depositPreapprovalCid: requireFlowRole(
              flow,
              ROLE_DEPOSIT_PREAPPROVAL
            ).contractId,
            extraArgs: EMPTY_EXTRA_ARGS,
          },
        };
  return buildSubmitRequest(
    ledger.userPartyId,
    commandId,
    exerciseCommand,
    flow
  );
}

/** Release (locking) or Mint (burning). A replay-node race resubmits with a new flow, same command id. */
export async function cantonRedeem(
  ledger: CantonLedger,
  disclosure: DisclosureServiceClient,
  input: CantonRedeemInput,
  options: { dryRun: boolean }
): Promise<{ commandId: string; transaction: LedgerTransaction } | undefined> {
  const canton = cantonLeg(input.deployment);
  const flowName: FlowName = canton.mode === "locking" ? "release" : "mint";
  const flowParameters = {
    manager: canton.manager,
    digest: input.parsedVaa.digest,
    recipient: input.recipient,
  };
  const commandId = redeemCommandId(input.parsedVaa.digest);

  for (
    let attemptNumber = 1;
    attemptNumber <= REDEEM_SUBMIT_MAX_ATTEMPTS;
    attemptNumber++
  ) {
    const flow = await fetchRedeemFlow(disclosure, flowName, flowParameters);
    const request = buildRedeemRequest(ledger, canton, input, flow, commandId);
    await checkFlowMatchesRegistry(flow, canton, disclosure);
    if (options.dryRun) {
      console.log(
        `  [dry-run] would submit command ${commandId} with ${flow.disclosures.length} disclosed contracts`
      );
      return undefined;
    }
    try {
      return {
        commandId,
        transaction: await ledger.submitAndWaitForTransaction(request),
      };
    } catch (err) {
      const isLastAttempt = attemptNumber === REDEEM_SUBMIT_MAX_ATTEMPTS;
      if (
        isLastAttempt ||
        !INACTIVE_CONTRACT_PATTERN.test((err as Error).message)
      )
        throw err;
      console.log(
        `  a disclosed contract changed (${(err as Error).message}); new flow, attempt ${attemptNumber + 1}`
      );
    }
  }
  throw new Error(
    `redeem: no attempt left after ${REDEEM_SUBMIT_MAX_ATTEMPTS}`
  );
}
