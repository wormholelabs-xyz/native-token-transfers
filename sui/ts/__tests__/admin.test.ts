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

  describe("setThreshold", () => {
    const newThreshold = 5;

    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(
          mockNttState({
            adminCapId:
              "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
          })
        )
        .mockResolvedValueOnce(
          mockSuiObject(
            "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef::ntt::State",
            {}
          )
        );
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
      // Create a fresh instance to avoid cached adminCapId
      const freshMockClient = mockSuiClient();
      const freshSuiNtt = new SuiNtt("Testnet", "Sui", freshMockClient, {
        ntt: TEST_CONTRACTS.ntt,
        coreBridge: TEST_CONTRACTS.coreBridge,
      });

      // Mock admin cap not found
      freshMockClient.getObject.mockResolvedValueOnce(
        mockNttState({ adminCapId: null })
      );

      const txGenerator = freshSuiNtt.setThreshold(3);
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
