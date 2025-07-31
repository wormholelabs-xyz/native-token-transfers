import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockCoinMetadata,
  mockAttestation,
  mockNttState,
  mockSuiObject,
  TEST_ADDRESSES,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Transfer Operations", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("transfer", () => {
    const transferAmount = 1000000000n; // 1 SUI in lamports
    const sender = TEST_ADDRESSES.USER;
    const destination = {
      chain: "Ethereum" as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1),
        toUniversalAddress: () => ({
          toUint8Array: () => new Uint8Array(32).fill(1),
        }),
      },
    } as any;
    const options = { queue: false };

    beforeEach(() => {
      // Mock package ID extraction
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject("0xpackage::ntt::State", {}))
        .mockResolvedValueOnce(mockCoinMetadata());

      // Mock coin metadata query
      mockClient.getCoinMetadata.mockResolvedValue(mockCoinMetadata());
    });

    it("should create transfer transaction with correct parameters", async () => {
      const txGenerator = suiNtt.transfer(
        sender as any,
        transferAmount,
        destination,
        options
      );
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("NTT Transfer");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");

      expect(mockClient.getCoinMetadata).toHaveBeenCalledWith({
        coinType: TEST_CONTRACTS.ntt.token,
      });
    });

    it("should handle queued transfer option", async () => {
      const queuedOptions = { queue: true };

      const txGenerator = suiNtt.transfer(
        sender as any,
        transferAmount,
        destination,
        queuedOptions
      );
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("NTT Transfer");
    });

    it("should throw error when coin metadata not found", async () => {
      mockClient.getCoinMetadata.mockResolvedValue(null);

      const txGenerator = suiNtt.transfer(
        sender as any,
        transferAmount,
        destination,
        options
      );
      await expect(txGenerator.next()).rejects.toThrow(
        "CoinMetadata not found for SUI"
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

      const txGenerator = customSuiNtt.transfer(
        sender as any,
        transferAmount,
        destination,
        options
      );
      await expect(txGenerator.next()).rejects.toThrow(
        "Transfer not yet implemented for token: 0xabc::custom::TOKEN"
      );
    });
  });

  describe("redeem", () => {
    it("should create redeem transaction skeleton", async () => {
      const attestation = mockAttestation();

      const txGenerator = suiNtt.redeem([attestation]);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Redeem NTT Transfer");
      expect(unsignedTx.network).toBe("Testnet");
      expect(unsignedTx.chain).toBe("Sui");
    });

    it("should handle multiple attestations", async () => {
      const attestations = [
        mockAttestation("Ethereum"),
        mockAttestation("Solana"),
      ];

      const txGenerator = suiNtt.redeem(attestations);
      const { value: unsignedTx } = await txGenerator.next();

      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe("Redeem NTT Transfer");
    });
  });

  describe("isRelayingAvailable", () => {
    it("should return false for all chains", async () => {
      const ethereumResult = await suiNtt.isRelayingAvailable("Ethereum");
      expect(ethereumResult).toBe(false);

      const solanaResult = await suiNtt.isRelayingAvailable("Solana");
      expect(solanaResult).toBe(false);
    });
  });

  describe("transfer status functions", () => {
    const attestation = mockAttestation();

    describe("getIsApproved", () => {
      beforeEach(() => {
        // Mock inbox item account lookup
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: false,
              release_after: null,
            },
          })
        );
      });

      it("should return true when transfer is approved (initialized)", async () => {
        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(true);
      });

      it("should return false when inbox item does not exist", async () => {
        mockClient.getObject.mockResolvedValue({ data: null });

        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(false);
      });
    });

    describe("getIsExecuted", () => {
      it("should return true when transfer is executed (tokens released)", async () => {
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: true,
              release_after: null,
            },
          })
        );

        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(true);
      });

      it("should return false when transfer not executed", async () => {
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: false,
              release_after: null,
            },
          })
        );

        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(false);
      });
    });

    describe("getIsTransferInboundQueued", () => {
      it("should return true when transfer is queued", async () => {
        const queuedTime = Date.now() + 12 * 60 * 60 * 1000; // 12 hours from now
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: false,
              release_after: queuedTime.toString(),
            },
          })
        );

        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(true);
      });

      it("should return false when transfer is not queued", async () => {
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: false,
              release_after: null,
            },
          })
        );

        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(false);
      });
    });
  });

  describe("queued transfer management", () => {
    const message = {
      id: new Uint8Array(32).fill(1),
      sender: new Uint8Array(32).fill(2),
      payload: {
        recipientAddress: new Uint8Array(32).fill(3),
        amount: "1000000000",
      },
    };

    describe("getInboundQueuedTransfer", () => {
      it("should return queued transfer details", async () => {
        const queuedTime = Date.now() + 12 * 60 * 60 * 1000; // 12 hours from now
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            recipient: "0x" + "3".repeat(64),
            amount: "1000000000",
            release_status: {
              released: false,
              release_after: queuedTime.toString(),
            },
          })
        );

        const result = await suiNtt.getInboundQueuedTransfer(
          "Ethereum",
          message as any
        );

        expect(result).not.toBeNull();
        expect(result?.recipient).toBe("0x" + "3".repeat(64));
        expect(result?.amount).toBe(1000000000n);
        expect(result?.rateLimitExpiryTimestamp).toBe(BigInt(queuedTime));
      });

      it("should return null when transfer is not queued", async () => {
        mockClient.getObject.mockResolvedValue(
          mockSuiObject("0xinbox::item::InboxItem", {
            init: true,
            release_status: {
              released: false,
              release_after: null,
            },
          })
        );

        const result = await suiNtt.getInboundQueuedTransfer(
          "Ethereum",
          message as any
        );
        expect(result).toBeNull();
      });
    });

    describe("completeInboundQueuedTransfer", () => {
      beforeEach(() => {
        // Mock state and inbox item for completion
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState({ mode: "Locking" })) // NTT state
          .mockResolvedValueOnce(
            mockSuiObject("0xinbox::item::InboxItem", {
              init: true,
              recipient: "0x" + "3".repeat(64),
              amount: "1000000000",
              release_status: {
                released: false,
                release_after: (Date.now() - 1000).toString(), // Past time, ready for completion
              },
            })
          );
      });

      it("should create complete queued transfer transaction", async () => {
        const txGenerator = suiNtt.completeInboundQueuedTransfer(
          "Ethereum",
          message as any
        );
        const { value: unsignedTx } = await txGenerator.next();

        expect(unsignedTx).toBeDefined();
        expect(unsignedTx.description).toBe("Complete Inbound Queued Transfer");
        expect(unsignedTx.network).toBe("Testnet");
        expect(unsignedTx.chain).toBe("Sui");
      });

      it("should throw error when transfer is not queued", async () => {
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState())
          .mockResolvedValueOnce(
            mockSuiObject("0xinbox::item::InboxItem", {
              init: true,
              release_status: {
                released: false,
                release_after: null, // Not queued
              },
            })
          );

        const txGenerator = suiNtt.completeInboundQueuedTransfer(
          "Ethereum",
          message as any
        );
        await expect(txGenerator.next()).rejects.toThrow(
          "Transfer is not queued"
        );
      });
    });
  });
});
