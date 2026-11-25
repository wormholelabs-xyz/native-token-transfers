import { CORE_PACKAGE_ID_0_12_3 } from "@wormhole-foundation/sdk-canton";
import type { NttAnchorPolicy, NttFlow } from "../src/index.js";
import {
  NTT_PACKAGE_ID_0_7_0,
  NttAnchorError,
  flowCoreState,
  parseFlowResponse,
  parseManagersResponse,
  reserveCustodianFor,
  verifyFlow,
} from "../src/index.js";
import { SANDBOX } from "./fixtures/sandbox.js";
import { MANAGERS, SETUP, copy } from "./helpers.js";

function policyOf(deployment: "burning" | "locking"): NttAnchorPolicy {
  return {
    guardianGovernance: SETUP.guardianGovernance,
    managerAddress: SETUP[deployment].managerAddress,
    reserveCustodian: SETUP[deployment].reserveCustodian,
    nttPackageIds: [NTT_PACKAGE_ID_0_7_0],
    corePackageIds: [CORE_PACKAGE_ID_0_12_3],
    instrument: {
      admin: SETUP[deployment].instrumentAdmin,
      id: SETUP[deployment].instrumentId,
    },
  };
}

const flows = {
  burnTransfer: () => parseFlowResponse(SANDBOX.burnTransfer.flow, "transfer"),
  lockTransfer: () => parseFlowResponse(SANDBOX.lockTransfer.flow, "transfer"),
  mint: () => parseFlowResponse(SANDBOX.mint.flow, "mint"),
  release: () => parseFlowResponse(SANDBOX.release.flow, "release"),
};

describe("verifyFlow on recorded flows", () => {
  it("passes the transfer flow of a burn/mint deployment with a third-party admin", () => {
    const verified = verifyFlow(
      flows.burnTransfer(),
      MANAGERS,
      policyOf("burning")
    );
    expect(verified.manager).toMatchObject({
      tokenConfig: "BurnMint",
      tokenDecimals: 8,
      admin: SETUP.burning.admin,
      reserveCustodian: SETUP.burning.admin,
      managerAddress: SETUP.burning.managerAddress,
      paused: false,
    });
    expect(verified.disclosures.map((entry) => entry.role)).toEqual([
      "NttManager",
      "transceiver Emitter",
      "CoreState",
      "committed canonical CoinFactory",
    ]);
    expect(verified.ignoredRoles).toEqual([]);
    expect(flowCoreState(verified).messageFee).toBe(0n);
  });

  it("passes the flows of a lock/unlock deployment under gg custody", () => {
    const transfer = verifyFlow(
      flows.lockTransfer(),
      MANAGERS,
      policyOf("locking")
    );
    expect(transfer.manager).toMatchObject({
      tokenConfig: "LockUnlock",
      admin: SETUP.guardianGovernance,
      reserveCustodian: SETUP.nttCustody,
    });
    // The first lock: the LockedLedger has no custody pot yet.
    expect(transfer.roles.has("custody pot Holding")).toBe(false);
    const release = verifyFlow(flows.release(), MANAGERS, policyOf("locking"), {
      recipient: SETUP.user,
    });
    expect(release.roles.has("custody pot Holding")).toBe(true);
    expect(release.roles.has("recipient TransferPreapproval")).toBe(true);
    const mint = verifyFlow(flows.mint(), MANAGERS, policyOf("burning"), {
      recipient: SETUP.user,
    });
    expect(mint.roles.has("DepositPreapproval")).toBe(true);
  });

  it("reads the manager from the blob, not from the served payload", () => {
    const flow = flows.burnTransfer();
    flow.disclosures[0] = {
      ...flow.disclosures[0]!,
      payload: {
        ...(flow.disclosures[0]!.payload as object),
        reserveCustodian: SETUP.other,
      },
    };
    expect(
      verifyFlow(flow, MANAGERS, policyOf("burning")).manager.reserveCustodian
    ).toBe(SETUP.burning.admin);
  });

  it("does not attach a role it does not know", () => {
    const flow = flows.burnTransfer();
    flow.disclosures.push({ ...flow.disclosures[2]!, role: "new role" });
    const verified = verifyFlow(flow, MANAGERS, policyOf("burning"));
    expect(verified.ignoredRoles).toEqual(["new role"]);
    expect(verified.disclosures).toHaveLength(4);
  });
});

describe("layer 1: pinned package ids", () => {
  it("refuses an NttManager of another ntt package", () => {
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        nttPackageIds: ["ab".repeat(32)],
      })
    ).toThrow(/layer 1: NttManager .* has package 43e1b206.*; pinned: abab/);
  });

  it("refuses a core contract of another core package", () => {
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        corePackageIds: ["cd".repeat(32)],
      })
    ).toThrow(/layer 1: transceiver Emitter .* has package 32abbcd2/);
  });
});

describe("layer 2: the /v1/managers cross-check", () => {
  const managersWith = (change: (entry: Record<string, unknown>) => void) => {
    const response = copy(SANDBOX.managers);
    for (const entry of response.managers)
      if (entry.managerAddress === SETUP.burning.managerAddress)
        change(entry as never);
    return parseManagersResponse(response);
  };

  it("refuses a manager that /v1/managers does not list", () => {
    const managers = MANAGERS.filter(
      (entry) => entry.managerAddress !== SETUP.burning.managerAddress
    );
    expect(() =>
      verifyFlow(flows.burnTransfer(), managers, policyOf("burning"))
    ).toThrow(/layer 2: \/v1\/managers lists 0 managers with address/);
  });

  it("refuses another contract id, mode or custodian in the summary", () => {
    expect(() =>
      verifyFlow(
        flows.burnTransfer(),
        managersWith(
          (entry) =>
            (entry["contractId"] = SANDBOX.managers.managers.find(
              (other) => other.managerAddress !== SETUP.burning.managerAddress
            )!.contractId)
        ),
        policyOf("burning")
      )
    ).toThrow(/layer 2: the NttManager contract id of the flow differs/);
    expect(() =>
      verifyFlow(
        flows.burnTransfer(),
        managersWith((entry) => (entry["tokenConfig"] = "LockUnlock")),
        policyOf("burning")
      )
    ).toThrow(/layer 2: \/v1\/managers has tokenConfig LockUnlock/);
    expect(() =>
      verifyFlow(
        flows.burnTransfer(),
        managersWith((entry) => (entry["reserveCustodian"] = SETUP.other)),
        policyOf("burning")
      )
    ).toThrow(/layer 2: \/v1\/managers has reserveCustodian/);
  });
});

describe("layer 3: the NttManager blob", () => {
  it("refuses a manager that the configured gg does not sign", () => {
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        guardianGovernance: SETUP.other,
      })
    ).toThrow(/NttManager .* is not signed by SdkOther/);
  });

  it("refuses a manager that the configured custodian does not sign", () => {
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        reserveCustodian: SETUP.other,
      })
    ).toThrow(/is not signed by SdkOther/);
  });

  it("refuses nttCustody as custodian of a manager whose admin is not gg, though nttCustody signs it", () => {
    expect(SETUP.burning.reserveCustodian).not.toBe(SETUP.nttCustody);
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        reserveCustodian: SETUP.nttCustody,
      })
    ).toThrow(
      /layer 3: the NttManager reserveCustodian SdkBurnAdmin.* differs from the configured SdkNttCustody/
    );
  });

  it("refuses another deployment's manager", () => {
    // The service names the locking deployment's manager for the burning deployment.
    const flow = flows.burnTransfer();
    const lockManager = flows.lockTransfer().disclosures[0]!;
    flow.disclosures[0] = lockManager;
    expect(() =>
      verifyFlow(flow, MANAGERS, {
        ...policyOf("burning"),
        reserveCustodian: SETUP.nttCustody,
      })
    ).toThrow(/layer 3: the NttManager has address .*, not the configured/);
  });

  it("refuses a blob that is not the disclosed contract's", () => {
    const flow = flows.burnTransfer();
    flow.disclosures[0] = {
      ...flow.disclosures[0]!,
      createdEventBlob: flows.lockTransfer().disclosures[0]!.createdEventBlob,
    };
    expect(() => verifyFlow(flow, MANAGERS, policyOf("burning"))).toThrow(
      NttAnchorError
    );
    expect(() => verifyFlow(flow, MANAGERS, policyOf("burning"))).toThrow(
      /the blob is for contract/
    );
  });

  it("refuses a mode or an instrument other than the configured one", () => {
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        tokenConfig: "LockUnlock",
      })
    ).toThrow(/tokenConfig BurnMint does not match the configured LockUnlock/);
    expect(() =>
      verifyFlow(flows.burnTransfer(), MANAGERS, {
        ...policyOf("burning"),
        instrument: { admin: SETUP.guardianGovernance, id: "other" },
      })
    ).toThrow(/instrument differs from the configured token/);
  });

  it("checks the custodian relation of Manager.daml", () => {
    const gg = SETUP.guardianGovernance;
    expect(
      reserveCustodianFor({
        guardianGovernance: gg,
        nttCustody: SETUP.nttCustody,
        admin: gg,
      })
    ).toBe(SETUP.nttCustody);
    expect(
      reserveCustodianFor({
        guardianGovernance: gg,
        nttCustody: SETUP.nttCustody,
        admin: SETUP.other,
      })
    ).toBe(SETUP.other);
  });
});

describe("the other roles of a flow", () => {
  const swapRole = (
    flow: NttFlow,
    role: string,
    from: NttFlow,
    fromRole: string
  ) => {
    const index = flow.disclosures.findIndex((entry) => entry.role === role);
    flow.disclosures[index] = {
      ...from.disclosures.find((entry) => entry.role === fromRole)!,
      role,
    };
    return flow;
  };

  it("refuses a missing role, naming it", () => {
    const flow = flows.lockTransfer();
    flow.missingRoles.push("committed TransferFactory");
    expect(() => verifyFlow(flow, MANAGERS, policyOf("locking"))).toThrow(
      /the transfer flow cannot supply: committed TransferFactory; nothing has been submitted/
    );
  });

  it("refuses a missing recipient preapproval when the CoinFactory needs it", () => {
    expect(() =>
      verifyFlow(
        parseFlowResponse(SANDBOX.releaseMissingPreapproval, "release"),
        MANAGERS,
        policyOf("locking"),
        {
          recipient: SETUP.other,
        }
      )
    ).toThrow(/cannot supply: recipient TransferPreapproval/);
  });

  it("refuses an absent required role", () => {
    const flow = flows.burnTransfer();
    flow.disclosures = flow.disclosures.filter(
      (entry) => entry.role !== "transceiver Emitter"
    );
    expect(() => verifyFlow(flow, MANAGERS, policyOf("burning"))).toThrow(
      /has no "transceiver Emitter" role/
    );
  });

  it("refuses a factory that the manager does not commit", () => {
    const flow = swapRole(
      flows.lockTransfer(),
      "committed TransferFactory",
      flows.lockTransfer(),
      "CoreState"
    );
    expect(() => verifyFlow(flow, MANAGERS, policyOf("locking"))).toThrow(
      /committed TransferFactory .* is not the factory the NttManager commits/
    );
  });

  it("refuses a core contract under another template, and a preapproval of another owner", () => {
    const wrongTemplate = swapRole(
      flows.burnTransfer(),
      "CoreState",
      flows.burnTransfer(),
      "transceiver Emitter"
    );
    expect(() =>
      verifyFlow(wrongTemplate, MANAGERS, policyOf("burning"))
    ).toThrow(
      /CoreState has template .*Emitter, expected Wormhole.Core.State:CoreState/
    );
    // The custodian preapproval is the user's, not the custodian's.
    const release = flows.release();
    const userPreapproval = release.disclosures.find(
      (entry) => entry.role === "recipient TransferPreapproval"
    )!;
    const transfer = flows.lockTransfer();
    const index = transfer.disclosures.findIndex(
      (entry) => entry.role === "custodian TransferPreapproval"
    );
    transfer.disclosures[index] = {
      ...userPreapproval,
      role: "custodian TransferPreapproval",
    };
    expect(() => verifyFlow(transfer, MANAGERS, policyOf("locking"))).toThrow(
      /custodian TransferPreapproval .* is not signed by SdkNttCustody/
    );
  });

  it("refuses a DepositPreapproval of another party than the recipient", () => {
    expect(() =>
      verifyFlow(flows.mint(), MANAGERS, policyOf("burning"), {
        recipient: SETUP.other,
      })
    ).toThrow(/DepositPreapproval .* is not signed by SdkOther/);
  });
});
