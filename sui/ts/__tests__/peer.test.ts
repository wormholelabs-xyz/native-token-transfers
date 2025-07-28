import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockNttState, 
  mockPeerData,
  mockSuiObject,
  TEST_CONTRACTS,
  TEST_CHAIN_IDS
} from './mocks';

describe('SuiNtt Peer Management Functions', () => {
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

  describe('getPeer', () => {
    beforeEach(() => {
      mockClient.getObject.mockResolvedValue(mockNttState());
    });

    it('should return peer data for existing peer', async () => {
      const peerData = mockPeerData(TEST_CHAIN_IDS.ETHEREUM);
      mockClient.getDynamicFieldObject.mockResolvedValue(peerData);

      const peer = await suiNtt.getPeer('Ethereum');
      
      expect(peer).not.toBeNull();
      expect(peer?.address.chain).toBe('Ethereum');
      expect(peer?.tokenDecimals).toBe(6);
      expect(peer?.inboundLimit).toBe(500000000000n);
      
      expect(mockClient.getDynamicFieldObject).toHaveBeenCalledWith({
        parentId: 'mock-peers-table-id',
        name: {
          type: 'u16',
          value: TEST_CHAIN_IDS.ETHEREUM
        }
      });
    });

    it('should return null for non-existent peer', async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      const peer = await suiNtt.getPeer('Ethereum');
      expect(peer).toBeNull();
    });

    it('should handle different chain IDs correctly', async () => {
      const solanaChainId = TEST_CHAIN_IDS.SOLANA;
      const peerData = mockPeerData(solanaChainId);
      mockClient.getDynamicFieldObject.mockResolvedValue(peerData);

      const peer = await suiNtt.getPeer('Solana');
      
      expect(peer).not.toBeNull();
      expect(peer?.address.chain).toBe('Solana');
      expect(mockClient.getDynamicFieldObject).toHaveBeenCalledWith({
        parentId: 'mock-peers-table-id',
        name: {
          type: 'u16',
          value: solanaChainId
        }
      });
    });

    it('should handle invalid peer data gracefully', async () => {
      const invalidPeerData = {
        data: {
          content: {
            dataType: 'invalidType',
            fields: {}
          }
        }
      };
      mockClient.getDynamicFieldObject.mockResolvedValue(invalidPeerData);

      const peer = await suiNtt.getPeer('Ethereum');
      expect(peer).toBeNull();
    });

    it('should throw error when state fetch fails', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getPeer('Ethereum')).rejects.toThrow('Failed to fetch NTT state object');
    });

    it('should handle RPC errors gracefully', async () => {
      mockClient.getDynamicFieldObject.mockRejectedValue(new Error('RPC error'));

      const peer = await suiNtt.getPeer('Ethereum');
      expect(peer).toBeNull();
    });
  });

  describe('setPeer', () => {
    const peerAddress = {
      chain: 'Ethereum' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1)
      }
    } as any;
    const tokenDecimals = 6;
    const inboundLimit = 1000000n;

    beforeEach(() => {
      // Mock required state and objects
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' })) // getAdminCapId
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {})) // getPackageId
        .mockResolvedValueOnce(mockSuiObject('0xwormhole::state::State', {})); // wormhole package ID
    });

    it('should create setPeer transaction with correct parameters', async () => {
      const txGenerator = suiNtt.setPeer(peerAddress, tokenDecimals, inboundLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Peer');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle different chain types', async () => {
      const solanaPeer = {
        chain: 'Solana' as const,
        address: {
          toUint8Array: () => new Uint8Array(32).fill(2)
        }
      } as any;
      
      const txGenerator = suiNtt.setPeer(solanaPeer, tokenDecimals, inboundLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Peer');
    });

    it('should handle large inbound limits', async () => {
      const largeLimit = BigInt('18446744073709551615'); // Max uint64
      
      const txGenerator = suiNtt.setPeer(peerAddress, tokenDecimals, largeLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Peer');
    });

    it('should handle zero inbound limit', async () => {
      const zeroLimit = 0n;
      
      const txGenerator = suiNtt.setPeer(peerAddress, tokenDecimals, zeroLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Peer');
    });

    it('should handle different token decimals', async () => {
      const decimals18 = 18;
      
      const txGenerator = suiNtt.setPeer(peerAddress, decimals18, inboundLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Peer');
    });

    it('should throw error when address conversion fails', async () => {
      const invalidPeer = {
        chain: 'Ethereum' as const,
        address: {
          toUint8Array: () => { throw new Error('Address conversion failed'); }
        }
      } as any;
      
      const txGenerator = suiNtt.setPeer(invalidPeer, tokenDecimals, inboundLimit);
      await expect(txGenerator.next()).rejects.toThrow('Failed to create setPeer transaction');
    });
  });

  describe('getTransceiverPeer', () => {
    beforeEach(() => {
      mockClient.getObject.mockResolvedValue({
        data: {
          content: {
            dataType: 'moveObject',
            fields: {
              peers: {
                fields: {
                  id: { id: 'transceiver-peers-table-id' },
                }
              }
            }
          }
        }
      });
    });

    it('should return transceiver peer for existing peer', async () => {
      const peerData = {
        data: {
          content: {
            dataType: 'moveObject',
            fields: {
              value: {
                fields: {
                  value: {
                    fields: {
                      data: Array.from(new Uint8Array(32).fill(1))
                    }
                  }
                }
              }
            }
          }
        }
      };
      mockClient.getDynamicFieldObject.mockResolvedValue(peerData);

      const peer = await suiNtt.getTransceiverPeer(0, 'Ethereum');
      
      expect(peer).not.toBeNull();
      expect(peer?.chain).toBe('Ethereum');
    });

    it('should return null for non-supported transceiver index', async () => {
      const peer = await suiNtt.getTransceiverPeer(1, 'Ethereum');
      expect(peer).toBeNull();
    });

    it('should return null when transceiver state ID not found', async () => {
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

      const peer = await suiNttWithoutTransceiver.getTransceiverPeer(0, 'Ethereum');
      expect(peer).toBeNull();
    });

    it('should return null for non-existent peer', async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      const peer = await suiNtt.getTransceiverPeer(0, 'Ethereum');
      expect(peer).toBeNull();
    });

    it('should handle RPC errors gracefully', async () => {
      mockClient.getDynamicFieldObject.mockRejectedValue(new Error('RPC error'));

      const peer = await suiNtt.getTransceiverPeer(0, 'Ethereum');
      expect(peer).toBeNull();
    });
  });

  describe('setTransceiverPeer', () => {
    const peerAddress = {
      chain: 'Ethereum' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1)
      }
    } as any;

    beforeEach(() => {
      // Mock transceiver state
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject('0xtransceiver::package::State', {})) // package ID extraction
        .mockResolvedValueOnce({ // transceiver state
          data: {
            content: {
              dataType: 'moveObject',
              fields: {
                admin_cap_id: 'transceiver-admin-cap-id'
              }
            }
          }
        })
        .mockResolvedValueOnce(mockNttState()) // getPackageId
        .mockResolvedValueOnce(mockSuiObject('0xwormhole::state::State', {})); // wormhole package ID
    });

    it('should create setTransceiverPeer transaction', async () => {
      const txGenerator = suiNtt.setTransceiverPeer(0, peerAddress);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Transceiver Peer');
    });

    it('should throw error for unsupported transceiver index', async () => {
      const txGenerator = suiNtt.setTransceiverPeer(1, peerAddress);
      await expect(txGenerator.next()).rejects.toThrow('Only transceiver index 0 (wormhole) is currently supported');
    });

    it('should throw error when wormhole transceiver not found', async () => {
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

      const txGenerator = suiNttWithoutTransceiver.setTransceiverPeer(0, peerAddress);
      await expect(txGenerator.next()).rejects.toThrow('Wormhole transceiver not found in contracts');
    });
  });

  describe('edge cases and error handling', () => {
    it('should handle network errors in peer queries', async () => {
      mockClient.getObject.mockRejectedValue(new Error('Network error'));

      await expect(suiNtt.getPeer('Ethereum')).rejects.toThrow('Network error');
    });

    it('should handle malformed peer data', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      
      const malformedPeerData = {
        data: {
          content: {
            dataType: 'moveObject',
            fields: {
              value: {
                fields: {
                  address: null, // Invalid address
                  token_decimals: 'invalid' // Invalid decimals
                }
              }
            }
          }
        }
      };
      mockClient.getDynamicFieldObject.mockResolvedValue(malformedPeerData);

      // Should handle gracefully
      await expect(suiNtt.getPeer('Ethereum')).rejects.toBeDefined();
    });

    it('should handle concurrent peer operations', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData());

      // Multiple concurrent calls should all resolve correctly
      const promises = [
        suiNtt.getPeer('Ethereum'),
        suiNtt.getPeer('Solana'),
        suiNtt.getTransceiverPeer(0, 'Ethereum')
      ];

      const results = await Promise.all(promises);
      expect(results).toHaveLength(3);
      expect(results[0]).not.toBeNull();
      expect(results[1]).not.toBeNull();
    });
  });
});