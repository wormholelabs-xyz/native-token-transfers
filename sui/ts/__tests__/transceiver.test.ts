import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockNttState,
  mockTransceiverState,
  mockTransceiverPeerData,
  mockDynamicFields,
  mockTransceiverInfo,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Transceiver Management Functions', () => {
  let suiNtt: SuiNtt<'Testnet', 'Sui'>;
  let mockClient: jest.Mocked<any>;

  beforeEach(() => {
    mockClient = mockSuiClient();
    suiNtt = new SuiNtt(
      'Testnet',
      'Sui',
      mockClient,
      { 
        ntt: TEST_CONTRACTS.ntt,
        coreBridge: TEST_CONTRACTS.coreBridge
      }
    );
  });

  describe('getTransceiver', () => {
    it('should return wormhole transceiver for index 0', async () => {
      const transceiver = await suiNtt.getTransceiver(0);
      
      expect(transceiver).not.toBeNull();
      expect(typeof transceiver?.getTransceiverType).toBe('function');
      expect(typeof transceiver?.getAddress).toBe('function');
      expect(typeof transceiver?.setPeer).toBe('function');
      expect(typeof transceiver?.getPeer).toBe('function');
    });

    it('should return null for non-supported transceiver index', async () => {
      const transceiver = await suiNtt.getTransceiver(1);
      expect(transceiver).toBeNull();
    });

    it('should return null when wormhole transceiver not configured', async () => {
      const suiNttWithoutTransceiver = new SuiNtt(
        'Testnet',
        'Sui',
        mockClient,
        {
          ntt: {
            ...TEST_CONTRACTS.ntt,
            transceiver: {}
          },
          coreBridge: TEST_CONTRACTS.coreBridge
        }
      );

      const transceiver = await suiNttWithoutTransceiver.getTransceiver(0);
      expect(transceiver).toBeNull();
    });

    describe('transceiver interface', () => {
      let transceiver: any;

      beforeEach(async () => {
        transceiver = await suiNtt.getTransceiver(0);
      });

      it('should return correct transceiver type', async () => {
        const type = await transceiver.getTransceiverType();
        expect(type).toBe('wormhole');
      });

      it('should return transceiver address', async () => {
        mockClient.getObject.mockResolvedValue(mockTransceiverState());

        const address = await transceiver.getAddress();
        
        expect(address).toBeDefined();
        expect(address.chain).toBe('Sui');
        expect(address.address).toBeDefined();
      });

      it('should handle setPeer delegation', async () => {
        const peerAddress = {
          chain: 'Ethereum' as const,
          address: {
            toUint8Array: () => new Uint8Array(32).fill(1)
          }
        };

        // Mock the required objects for setTransceiverPeer
        mockClient.getObject
          .mockResolvedValueOnce(mockTransceiverState()) // package ID extraction
          .mockResolvedValueOnce(mockTransceiverState()) // transceiver state
          .mockResolvedValueOnce(mockNttState()) // getPackageId
          .mockResolvedValueOnce(mockTransceiverState()); // wormhole package ID

        const txGenerator = transceiver.setPeer(peerAddress);
        const { value: unsignedTx } = await txGenerator.next();
        
        expect(unsignedTx).toBeDefined();
        expect(unsignedTx.description).toBe('Set Transceiver Peer');
      });

      it('should handle getPeer delegation', async () => {
        mockClient.getObject.mockResolvedValue(mockTransceiverState());
        mockClient.getDynamicFieldObject.mockResolvedValue(mockTransceiverPeerData());

        const peer = await transceiver.getPeer('Ethereum');
        
        expect(peer).not.toBeNull();
        expect(peer.chain).toBe('Ethereum');
      });

      it('should throw error for setPauser', async () => {
        const txGenerator = transceiver.setPauser();
        await expect(txGenerator.next()).rejects.toThrow('setPauser not implemented for Sui transceiver');
      });

      it('should return null for getPauser', async () => {
        const pauser = await transceiver.getPauser();
        expect(pauser).toBeNull();
      });

      it('should throw error for receive', async () => {
        const txGenerator = transceiver.receive();
        await expect(txGenerator.next()).rejects.toThrow('receive not implemented for Sui transceiver');
      });
    });
  });

  describe('verifyAddresses', () => {
    beforeEach(() => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      mockClient.getDynamicFields.mockResolvedValue(mockDynamicFields());
      mockClient.getObject.mockResolvedValue(mockTransceiverInfo());
    });

    it('should verify NTT contracts and discover transceivers', async () => {
      const result = await suiNtt.verifyAddresses();
      
      // Should return differences or null if everything matches
      expect(result).toBeDefined();
    });

    it('should handle state object fetch failure', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      const result = await suiNtt.verifyAddresses();
      expect(result).toBeNull();
    });

    it('should handle dynamic fields query failure', async () => {
      mockClient.getDynamicFields.mockRejectedValue(new Error('Query failed'));

      const result = await suiNtt.verifyAddresses();
      expect(result).toBeNull();
    });

    it('should discover registered transceivers', async () => {
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // state object
        .mockResolvedValueOnce(mockTransceiverInfo()); // transceiver info

      await suiNtt.verifyAddresses();
      
      // Should process the transceiver registry
      expect(mockClient.getDynamicFields).toHaveBeenCalled();
    });

    it('should handle malformed transceiver info gracefully', async () => {
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // state object
        .mockResolvedValueOnce({ data: null }); // invalid transceiver info

      const result = await suiNtt.verifyAddresses();
      
      // Should handle gracefully
      expect(result).toBeDefined();
    });

    it('should return null when addresses match local config', async () => {
      // Mock exact match scenario
      const transceiverInfo = mockTransceiverInfo();
      transceiverInfo.data.content.fields.value.fields.state_object_id = TEST_CONTRACTS.ntt.transceiver?.wormhole;

      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // state object
        .mockResolvedValueOnce(transceiverInfo); // matching transceiver info

      const result = await suiNtt.verifyAddresses();
      
      // Should return null when everything matches
      expect(result).toBeNull();
    });
  });

  describe('error handling and edge cases', () => {
    it('should handle RPC errors gracefully', async () => {
      mockClient.getObject.mockRejectedValue(new Error('RPC error'));

      const result = await suiNtt.verifyAddresses();
      expect(result).toBeNull();
    });

    it('should handle concurrent transceiver operations', async () => {
      const promises = [
        suiNtt.getTransceiver(0),
        suiNtt.getTransceiver(1),
        suiNtt.verifyAddresses()
      ];

      const results = await Promise.all(promises);
      
      expect(results[0]).not.toBeNull(); // Index 0 transceiver
      expect(results[1]).toBeNull(); // Index 1 transceiver
      expect(results[2]).toBeDefined(); // Verify addresses result
    });

    it('should validate transceiver interface completeness', async () => {
      const transceiver = await suiNtt.getTransceiver(0);
      
      if (transceiver) {
        // Check all required methods exist
        const requiredMethods = [
          'getTransceiverType',
          'getAddress',
          'setPeer',
          'getPeer',
          'setPauser',
          'getPauser',
          'receive'
        ];

        for (const method of requiredMethods) {
          expect(typeof (transceiver as any)[method]).toBe('function');
        }
      }
    });

    it('should handle invalid transceiver state objects', async () => {
      mockClient.getObject.mockResolvedValue({
        data: {
          content: {
            dataType: 'package', // Wrong type
            fields: {}
          }
        }
      });

      const transceiver = await suiNtt.getTransceiver(0);
      
      if (transceiver) {
        await expect(transceiver.getAddress()).rejects.toBeDefined();
      }
    });

    it('should handle missing emitter cap in transceiver state', async () => {
      const stateWithoutEmitterCap = mockTransceiverState();
      (stateWithoutEmitterCap.data.content.fields as any).emitter_cap = undefined;
      
      mockClient.getObject.mockResolvedValue(stateWithoutEmitterCap);

      const transceiver = await suiNtt.getTransceiver(0);
      
      if (transceiver) {
        await expect(transceiver.getAddress()).rejects.toBeDefined();
      }
    });
  });
});