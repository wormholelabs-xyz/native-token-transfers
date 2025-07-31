import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockNttState,
  mockAdminCap,
  TEST_ADDRESSES,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Core State Functions", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("constructor", () => {
    it("should create SuiNtt instance with correct properties", () => {
      expect(suiNtt.network).toBe("Testnet");
      expect(suiNtt.chain).toBe("Sui");
      expect(suiNtt.provider).toBe(mockClient);
      expect(suiNtt.contracts.ntt?.manager).toBe(TEST_CONTRACTS.ntt.manager);
      expect(suiNtt.contracts.ntt?.token).toBe(TEST_CONTRACTS.ntt.token);
    });

    it("should throw error when NTT contracts not found", () => {
      expect(() => {
        new SuiNtt("Testnet", "Sui", mockClient, {
          coreBridge: TEST_CONTRACTS.coreBridge,
        });
      }).toThrow("NTT contracts not found");
    });

    it("should throw error when Core Bridge contract not found", () => {
      expect(() => {
        new SuiNtt("Testnet", "Sui", mockClient, {
          ntt: TEST_CONTRACTS.ntt,
        });
      }).toThrow("Core Bridge contract not found");
    });
  });

  describe("getMode", () => {
    it("should return locking mode when state has Locking variant", async () => {
      const state = mockNttState({ mode: "Locking" });
      mockClient.getObject.mockResolvedValue(state);

      const mode = await suiNtt.getMode();
      expect(mode).toBe("locking");
      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: TEST_CONTRACTS.ntt.manager,
        options: { showContent: true },
      });
    });

    it("should return burning mode when state has Burning variant", async () => {
      const state = mockNttState({ mode: "Burning" });
      mockClient.getObject.mockResolvedValue(state);

      const mode = await suiNtt.getMode();
      expect(mode).toBe("burning");
    });

    it("should throw error when state fetch fails", async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getMode()).rejects.toThrow(
        "Failed to fetch NTT state object"
      );
    });
  });

  describe("isPaused", () => {
    it("should return false as Sui NTT uses AdminCap access control", async () => {
      const result = await suiNtt.isPaused();
      expect(result).toBe(false);
    });
  });

  describe("getAdminCapId", () => {
    it("should return admin cap ID from state", async () => {
      const state = mockNttState({ adminCapId: "test-admin-cap-id" });
      mockClient.getObject.mockResolvedValue(state);

      const adminCapId = await suiNtt.getAdminCapId();
      expect(adminCapId).toBe("test-admin-cap-id");
    });

    it("should throw error when admin cap ID not found", async () => {
      const state = mockNttState({ adminCapId: null });
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getAdminCapId()).rejects.toThrow(
        "AdminCap ID not found in NTT state"
      );
    });
  });

  describe("getPackageId", () => {
    it("should extract package ID from state object type", async () => {
      const state = mockNttState();
      state.data.content.type = "0xabc123::ntt::State<0x2::sui::SUI>";
      mockClient.getObject.mockResolvedValue(state);

      const packageId = await suiNtt.getPackageId();
      expect(packageId).toBe("0xabc123");
    });

    it("should throw error when package ID cannot be extracted", async () => {
      const state = mockNttState();
      state.data.content.type = "invalid::format";
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getPackageId()).rejects.toThrow(
        "Could not extract package ID from state object type"
      );
    });
  });

  describe("getOwner", () => {
    it("should return owner address from AdminCap", async () => {
      const state = mockNttState({ adminCapId: "admin-cap-123" });
      const adminCap = mockAdminCap(TEST_ADDRESSES.ADMIN);

      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      const owner = await suiNtt.getOwner();
      expect(owner).toBe(TEST_ADDRESSES.ADMIN);

      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: "admin-cap-123",
        options: { showOwner: true },
      });
    });

    it("should throw error when AdminCap owner not found", async () => {
      const state = mockNttState({ adminCapId: "admin-cap-123" });
      const adminCap = { data: { owner: null } };

      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      await expect(suiNtt.getOwner()).rejects.toThrow(
        "Could not fetch AdminCap owner information"
      );
    });
  });

  describe("getPauser", () => {
    it("should return null as not implemented", async () => {
      const result = await suiNtt.getPauser();
      expect(result).toBeNull();
    });
  });

  describe("getThreshold", () => {
    it("should return threshold value from state", async () => {
      const state = mockNttState({ threshold: "5" });
      mockClient.getObject.mockResolvedValue(state);

      const threshold = await suiNtt.getThreshold();
      expect(threshold).toBe(5);
    });

    it("should throw error when state fetch fails", async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getThreshold()).rejects.toThrow(
        "Failed to fetch NTT state object"
      );
    });
  });

  describe("getTokenDecimals", () => {
    it("should return 9 for SUI token", async () => {
      const decimals = await suiNtt.getTokenDecimals();
      expect(decimals).toBe(9);
    });

    it("should throw error for non-SUI tokens", async () => {
      const customSuiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
        ntt: {
          ...TEST_CONTRACTS.ntt,
          token: "0xabc::custom::TOKEN",
        },
        coreBridge: TEST_CONTRACTS.coreBridge,
      });

      await expect(customSuiNtt.getTokenDecimals()).rejects.toThrow(
        "getTokenDecimals not yet implemented for token: 0xabc::custom::TOKEN"
      );
    });
  });

  describe("getCustodyAddress", () => {
    it("should return NTT manager address as custody address", async () => {
      const custodyAddress = await suiNtt.getCustodyAddress();
      expect(custodyAddress).toBe(TEST_CONTRACTS.ntt.manager);
    });
  });

  describe("getRateLimitDuration", () => {
    it("should return 24 hours in milliseconds", async () => {
      const duration = await suiNtt.getRateLimitDuration();
      expect(duration).toBe(BigInt(24 * 60 * 60 * 1000));
    });
  });
});
