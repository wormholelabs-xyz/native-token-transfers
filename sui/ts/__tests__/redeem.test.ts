import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockAttestation,
  mockSuiObject,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Redeem Function", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("redeem", () => {
    it("should create redeem transaction for valid attestation", async () => {
      // Mock required objects
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {})) // getPackageId
        .mockResolvedValueOnce(
          mockSuiObject("0xwormhole::wormhole_transceiver::State", {})
        ); // getWormholePackageId

      const attestation = mockAttestation("Ethereum");

      const txGenerator = suiNtt.redeem([attestation]);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Redeem NTT Transfer");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should handle multiple attestations", async () => {
      // Mock required objects
      mockClient.getObject.mockResolvedValue(
        mockSuiObject("0xpackage::ntt::State", {})
      );

      const attestations = [
        mockAttestation("Ethereum"),
        mockAttestation("Solana"),
      ];

      const txGenerator = suiNtt.redeem(attestations);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Redeem NTT Transfer");
    });

    it("should throw error for unsupported attestation type", async () => {
      const invalidAttestation = {
        ...mockAttestation("Ethereum"),
        payloadName: "UnsupportedType" as any,
      };

      const txGenerator = suiNtt.redeem([invalidAttestation]);
      await expect(txGenerator.next()).rejects.toThrow(
        "Unsupported attestation type: UnsupportedType"
      );
    });

    it("should throw error for non-SUI tokens", async () => {
      const customSuiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
        ntt: {
          ...TEST_CONTRACTS.ntt,
          token: "0xabc::custom::TOKEN",
        },
        coreBridge: TEST_CONTRACTS.coreBridge,
      });

      const attestation = mockAttestation("Ethereum");

      const txGenerator = customSuiNtt.redeem([attestation]);
      await expect(txGenerator.next()).rejects.toThrow(
        "Redeem not yet implemented for token: 0xabc::custom::TOKEN"
      );
    });

    it("should throw error when wormhole transceiver not configured", async () => {
      const suiNttWithoutTransceiver = new SuiNtt(
        "Testnet",
        "Sui",
        mockClient,
        {
          ntt: {
            ...TEST_CONTRACTS.ntt,
            transceiver: {},
          },
          coreBridge: TEST_CONTRACTS.coreBridge,
        }
      );

      // Mock package ID call
      mockClient.getObject.mockResolvedValueOnce(
        mockSuiObject("0xpackage::ntt::State", {})
      );

      const attestation = mockAttestation("Ethereum");

      const txGenerator = suiNttWithoutTransceiver.redeem([attestation]);
      await expect(txGenerator.next()).rejects.toThrow(
        "Wormhole transceiver not configured"
      );
    });
  });
});
