import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockNttState,
  mockSuiObject,
  mockPeerData,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Rate Limiting Functions", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("getCurrentOutboundCapacity", () => {
    it("should return current outbound capacity from state", async () => {
      const state = mockNttState({
        outboundCapacity: "750000000000",
        outboundLimit: "1000000000000",
      });
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();

      // Should return calculated capacity based on time passage
      expect(typeof capacity).toBe("bigint");
      expect(capacity).toBeGreaterThanOrEqual(750000000000n);

      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: TEST_CONTRACTS.ntt.manager,
        options: { showContent: true },
      });
    });

    it("should throw error when state fetch fails", async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toThrow(
        "Failed to fetch NTT state object"
      );
    });
  });

  describe("getOutboundLimit", () => {
    it("should return outbound limit from state", async () => {
      const state = mockNttState({
        outboundCapacity: "750000000000",
        outboundLimit: "1000000000000",
      });
      mockClient.getObject.mockResolvedValue(state);

      const limit = await suiNtt.getOutboundLimit();
      expect(limit).toBe(1000000000000n);
    });

    it("should handle zero limit", async () => {
      const state = mockNttState({
        outboundCapacity: "0",
        outboundLimit: "0",
      });
      mockClient.getObject.mockResolvedValue(state);

      const limit = await suiNtt.getOutboundLimit();
      expect(limit).toBe(0n);
    });
  });

  describe("setOutboundLimit", () => {
    const newLimit = 2000000000000n;

    it("should create setOutboundLimit transaction skeleton", async () => {
      const txGenerator = suiNtt.setOutboundLimit(newLimit);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Set Outbound Limit");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should handle zero limit", async () => {
      const zeroLimit = 0n;

      const txGenerator = suiNtt.setOutboundLimit(zeroLimit);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Set Outbound Limit");
    });
  });

  describe("getCurrentInboundCapacity", () => {
    beforeEach(() => {
      // Mock peer data and rate limit info
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject("0xwormhole::state::State", {})); // wormhole package ID

      mockClient.getDynamicFieldObject.mockResolvedValue(
        mockPeerData({
          inboundLimit: "2000000000000",
          inboundCapacity: "1500000000000",
        })
      );
    });

    it("should return current inbound capacity for a chain", async () => {
      const capacity = await suiNtt.getCurrentInboundCapacity("Ethereum");

      expect(typeof capacity).toBe("bigint");
      expect(capacity).toBeGreaterThanOrEqual(0n);
      expect(capacity).toBeLessThanOrEqual(2000000000000n); // Should not exceed limit
    });

    it("should throw error when peer not found", async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      await expect(
        suiNtt.getCurrentInboundCapacity("UnknownChain" as any)
      ).rejects.toThrow("No peer found");
    });
  });

  describe("getInboundLimit", () => {
    beforeEach(() => {
      // Mock peer data
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject("0xwormhole::state::State", {})); // wormhole package ID

      mockClient.getDynamicFieldObject.mockResolvedValue(
        mockPeerData({
          inboundLimit: "3000000000000",
        })
      );
    });

    it("should return inbound limit for a chain", async () => {
      const limit = await suiNtt.getInboundLimit("Ethereum");

      expect(limit).toBe(3000000000000n);
    });

    it("should throw error when peer not found", async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      await expect(
        suiNtt.getInboundLimit("UnknownChain" as any)
      ).rejects.toThrow("No peer found");
    });
  });

  describe("setInboundLimit", () => {
    const newLimit = 2000000000000n;

    beforeEach(() => {
      // Mock existing peer data
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: "admin-cap-id" })) // getAdminCapId
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {})) // getPackageId
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject("0xwormhole::state::State", {})); // wormhole package ID

      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData());
    });

    it("should create setInboundLimit transaction with existing peer", async () => {
      const txGenerator = suiNtt.setInboundLimit("Ethereum", newLimit);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Set Inbound Limit");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should throw error when peer not found", async () => {
      // Mock no existing peer
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      const txGenerator = suiNtt.setInboundLimit("Ethereum", newLimit);
      await expect(txGenerator.next()).rejects.toThrow(
        "No peer found for chain Ethereum"
      );
    });
  });

  describe("getRateLimitDuration", () => {
    it("should return 24 hours in milliseconds", async () => {
      const duration = await suiNtt.getRateLimitDuration();
      expect(duration).toBe(BigInt(24 * 60 * 60 * 1000)); // 24 hours in milliseconds
    });
  });
});
