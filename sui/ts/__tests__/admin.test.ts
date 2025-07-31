import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockNttState,
  mockSuiObject,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Admin Functions", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("pause", () => {
    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: "admin-cap-id" }))
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {}));
    });

    it("should create pause transaction", async () => {
      const txGenerator = suiNtt.pause();
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Pause NTT");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should require admin cap for pausing", async () => {
      // Mock admin cap not found
      mockClient.getObject.mockResolvedValueOnce(
        mockNttState({ adminCapId: null })
      );

      const txGenerator = suiNtt.pause();
      await expect(txGenerator.next()).rejects.toThrow(
        "AdminCap ID not found in NTT state"
      );
    });
  });

  describe("unpause", () => {
    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: "admin-cap-id" }))
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {}));
    });

    it("should create unpause transaction", async () => {
      const txGenerator = suiNtt.unpause();
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Unpause NTT");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should require admin cap for unpausing", async () => {
      // Mock admin cap not found
      mockClient.getObject.mockResolvedValueOnce(
        mockNttState({ adminCapId: null })
      );

      const txGenerator = suiNtt.unpause();
      await expect(txGenerator.next()).rejects.toThrow(
        "AdminCap ID not found in NTT state"
      );
    });
  });

  describe("setOwner", () => {
    const newOwner = "0x" + "1".repeat(64);

    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: "admin-cap-id" }))
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {}));
    });

    it("should create setOwner transaction", async () => {
      const txGenerator = suiNtt.setOwner(newOwner as any);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Transfer Ownership");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should require admin cap for ownership transfer", async () => {
      // Mock admin cap not found
      mockClient.getObject.mockResolvedValueOnce(
        mockNttState({ adminCapId: null })
      );

      const txGenerator = suiNtt.setOwner(newOwner as any);
      await expect(txGenerator.next()).rejects.toThrow(
        "AdminCap ID not found in NTT state"
      );
    });
  });

  describe("setPauser", () => {
    const newPauser = "0x" + "1".repeat(64);

    it("should throw not supported error for Sui", async () => {
      const txGenerator = suiNtt.setPauser(newPauser as any);
      await expect(txGenerator.next()).rejects.toThrow(
        "Pauser role not supported on Sui"
      );
    });
  });

  describe("setThreshold", () => {
    const newThreshold = 5;

    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: "admin-cap-id" }))
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {}));
    });

    it("should create setThreshold transaction with correct parameters", async () => {
      const txGenerator = suiNtt.setThreshold(newThreshold);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Set Threshold");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should require admin cap for threshold changes", async () => {
      // Mock admin cap not found
      mockClient.getObject.mockResolvedValueOnce(
        mockNttState({ adminCapId: null })
      );

      const txGenerator = suiNtt.setThreshold(3);
      await expect(txGenerator.next()).rejects.toThrow(
        "AdminCap ID not found in NTT state"
      );
    });
  });

  describe("getUpgradeCapId", () => {
    it("should return upgrade cap ID from state", async () => {
      const state = mockNttState({ upgradeCapId: "upgrade-cap-123" });
      mockClient.getObject.mockResolvedValue(state);

      const upgradeCapId = await suiNtt.getUpgradeCapId();
      expect(upgradeCapId).toBe("upgrade-cap-123");
    });

    it("should throw error when upgrade cap ID not found", async () => {
      const state = mockNttState({ upgradeCapId: null });
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getUpgradeCapId()).rejects.toThrow(
        "UpgradeCap ID not found in NTT state"
      );
    });
  });
});
