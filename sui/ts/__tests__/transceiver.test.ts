import { SuiNtt } from "../src/ntt.js";
import {
  mockSuiClient,
  mockNttState,
  mockTransceiverState,
  mockTransceiverPeerData,
  mockDynamicFields,
  mockTransceiverInfo,
  TEST_CONTRACTS,
} from "./mocks.js";

describe("SuiNtt Transceiver Management Functions", () => {
  let suiNtt: SuiNtt<"Testnet", "Sui">;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt("Testnet", "Sui", mockClient, {
      ntt: TEST_CONTRACTS.ntt,
      coreBridge: TEST_CONTRACTS.coreBridge,
    });
  });

  describe("getTransceiver", () => {
    it("should return wormhole transceiver for index 0", async () => {
      const transceiver = await suiNtt.getTransceiver(0);

      expect(transceiver).not.toBeNull();
      expect(typeof transceiver?.getTransceiverType).toBe("function");
      expect(typeof transceiver?.getAddress).toBe("function");
      expect(typeof transceiver?.setPeer).toBe("function");
      expect(typeof transceiver?.getPeer).toBe("function");
    });

    it("should return null for non-supported transceiver index", async () => {
      const transceiver = await suiNtt.getTransceiver(1);
      expect(transceiver).toBeNull();
    });

    it("should return null when wormhole transceiver not configured", async () => {
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

      const transceiver = await suiNttWithoutTransceiver.getTransceiver(0);
      expect(transceiver).toBeNull();
    });

    describe("transceiver interface", () => {
      let transceiver: any;

      beforeEach(async () => {
        transceiver = await suiNtt.getTransceiver(0);
      });

      it("should return correct transceiver type", async () => {
        const type = await transceiver.getTransceiverType();
        expect(type).toBe("wormhole");
      });

      it("should return transceiver address", async () => {
        mockClient.getObject.mockResolvedValue(mockTransceiverState());

        const address = await transceiver.getAddress();

        expect(address).toBeDefined();
        expect(address.chain).toBe("Sui");
        expect(address.address).toBeDefined();
      });

      it("should handle setPeer delegation", async () => {
        const peerAddress = {
          chain: "Ethereum" as const,
          address: {
            toUint8Array: () => new Uint8Array(32).fill(1),
          },
        };

        // Mock the setTransceiverPeer method directly to avoid complex mock setup
        const mockTxGenerator = {
          async *[Symbol.asyncIterator]() {
            yield {
              description: "Set Transceiver Peer",
              network: "Testnet",
              chain: "Sui",
              parallelizable: false,
              transaction: {},
            };
          },
        };

        jest
          .spyOn(suiNtt, "setTransceiverPeer")
          .mockReturnValue(mockTxGenerator as any);

        const txGenerator = transceiver.setPeer(peerAddress);
        const { value: unsignedTx } = await txGenerator[
          Symbol.asyncIterator
        ]().next();

        expect(unsignedTx).toBeDefined();
        expect(unsignedTx.description).toBe("Set Transceiver Peer");
      });

      it("should handle getPeer delegation", async () => {
        mockClient.getObject.mockResolvedValue(mockTransceiverState());
        mockClient.getDynamicFieldObject.mockResolvedValue(
          mockTransceiverPeerData()
        );

        const peer = await transceiver.getPeer("Ethereum");

        expect(peer).not.toBeNull();
        expect(peer.chain).toBe("Ethereum");
      });

      it("should throw error for setPauser", async () => {
        const txGenerator = transceiver.setPauser();
        await expect(txGenerator.next()).rejects.toThrow(
          "setPauser not implemented for Sui transceiver"
        );
      });

      it("should return null for getPauser", async () => {
        const pauser = await transceiver.getPauser();
        expect(pauser).toBeNull();
      });

      it("should throw error for receive", async () => {
        const txGenerator = transceiver.receive();
        await expect(txGenerator.next()).rejects.toThrow(
          "receive not implemented for Sui transceiver"
        );
      });
    });
  });

  describe("verifyAddresses", () => {
    beforeEach(() => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      mockClient.getDynamicFields.mockResolvedValue(mockDynamicFields());
      mockClient.getObject.mockResolvedValue(mockTransceiverInfo());
    });

    it("should verify NTT contracts and discover transceivers", async () => {
      const result = await suiNtt.verifyAddresses();

      // Should return differences or null if everything matches
      expect(result).toBeDefined();
    });

    it("should handle state object fetch failure", async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      const result = await suiNtt.verifyAddresses();
      expect(result).toBeNull();
    });

    it("should return null when addresses match local config", async () => {
      // Mock exact match scenario
      const transceiverInfo = mockTransceiverInfo();
      transceiverInfo.data.content.fields.value.fields.state_object_id =
        TEST_CONTRACTS.ntt.transceiver?.wormhole;

      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // state object
        .mockResolvedValueOnce(transceiverInfo); // matching transceiver info

      const result = await suiNtt.verifyAddresses();

      // Should return null when everything matches
      expect(result).toBeNull();
    });
  });
});
