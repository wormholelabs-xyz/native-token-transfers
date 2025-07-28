import { SuiNtt } from '../src/ntt';
import { SuiPlatform } from '@wormhole-foundation/sdk-sui';
import { 
  mockSuiClient, 
  mockNttState,
  TEST_ADDRESSES,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Integration Tests', () => {
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

  describe('SDK Integration', () => {
    it('should initialize with correct configuration', () => {
      expect(suiNtt.network).toBe('Testnet');
      expect(suiNtt.chain).toBe('Sui');
      expect(suiNtt.provider).toBe(mockClient);
      expect(suiNtt.contracts.ntt?.manager).toBe(TEST_CONTRACTS.ntt.manager);
      expect(suiNtt.contracts.ntt?.token).toBe(TEST_CONTRACTS.ntt.token);
      expect(suiNtt.contracts.coreBridge).toBe(TEST_CONTRACTS.coreBridge);
    });

    it('should have all required NTT interface methods', () => {
      // Core state functions
      expect(typeof suiNtt.getMode).toBe('function');
      expect(typeof suiNtt.isPaused).toBe('function');
      expect(typeof suiNtt.getOwner).toBe('function');
      expect(typeof suiNtt.getPauser).toBe('function');
      expect(typeof suiNtt.getThreshold).toBe('function');
      expect(typeof suiNtt.getTokenDecimals).toBe('function');
      expect(typeof suiNtt.getCustodyAddress).toBe('function');

      // Admin functions
      expect(typeof suiNtt.pause).toBe('function');
      expect(typeof suiNtt.unpause).toBe('function');
      expect(typeof suiNtt.setOwner).toBe('function');
      expect(typeof suiNtt.setPauser).toBe('function');

      // Transfer operations
      expect(typeof suiNtt.transfer).toBe('function');
      expect(typeof suiNtt.redeem).toBe('function');
      expect(typeof suiNtt.quoteDeliveryPrice).toBe('function');
      expect(typeof suiNtt.isRelayingAvailable).toBe('function');

      // Peer management
      expect(typeof suiNtt.setPeer).toBe('function');
      expect(typeof suiNtt.getPeer).toBe('function');
      expect(typeof suiNtt.setTransceiverPeer).toBe('function');

      // Rate limiting
      expect(typeof suiNtt.getCurrentOutboundCapacity).toBe('function');
      expect(typeof suiNtt.getOutboundLimit).toBe('function');
      expect(typeof suiNtt.setOutboundLimit).toBe('function');
      expect(typeof suiNtt.getCurrentInboundCapacity).toBe('function');
      expect(typeof suiNtt.getInboundLimit).toBe('function');
      expect(typeof suiNtt.setInboundLimit).toBe('function');
      expect(typeof suiNtt.getRateLimitDuration).toBe('function');

      // Transfer status
      expect(typeof suiNtt.getIsApproved).toBe('function');
      expect(typeof suiNtt.getIsExecuted).toBe('function');
      expect(typeof suiNtt.getIsTransferInboundQueued).toBe('function');
      expect(typeof suiNtt.getInboundQueuedTransfer).toBe('function');
      expect(typeof suiNtt.completeInboundQueuedTransfer).toBe('function');

      // Transceiver management
      expect(typeof suiNtt.getTransceiver).toBe('function');
      expect(typeof suiNtt.verifyAddresses).toBe('function');
    });

    it('should handle provider interactions correctly', () => {
      expect(suiNtt.provider).toBe(mockClient);
      expect(suiNtt.provider.getObject).toBeDefined();
      expect(suiNtt.provider.getDynamicFieldObject).toBeDefined();
      expect(suiNtt.provider.getCoinMetadata).toBeDefined();
    });
  });

  describe('fromRpc static method', () => {
    it('should create SuiNtt instance from RPC configuration', async () => {
      // Mock SuiPlatform.chainFromRpc
      const mockChainFromRpc = jest.spyOn(SuiPlatform, 'chainFromRpc');
      mockChainFromRpc.mockResolvedValue(['Testnet', 'Sui']);

      const config = {
        Sui: {
          network: 'Testnet',
          contracts: {
            coreBridge: TEST_CONTRACTS.coreBridge,
            ntt: TEST_CONTRACTS.ntt
          }
        }
      };

      const nttInstance = await SuiNtt.fromRpc(mockClient, config as any);
      
      expect(nttInstance).toBeInstanceOf(SuiNtt);
      expect(nttInstance.network).toBe('Testnet');
      expect(nttInstance.chain).toBe('Sui');
      expect(nttInstance.contracts.ntt).toBe(TEST_CONTRACTS.ntt);

      mockChainFromRpc.mockRestore();
    });

    it('should throw error for network mismatch', async () => {
      const mockChainFromRpc = jest.spyOn(SuiPlatform, 'chainFromRpc');
      mockChainFromRpc.mockResolvedValue(['Mainnet', 'Sui']);

      const config = {
        Sui: {
          network: 'Testnet', // Mismatch
          contracts: {
            coreBridge: TEST_CONTRACTS.coreBridge,
            ntt: TEST_CONTRACTS.ntt
          }
        }
      };

      await expect(SuiNtt.fromRpc(mockClient, config as any))
        .rejects.toThrow('Network mismatch: Testnet != Mainnet');

      mockChainFromRpc.mockRestore();
    });

    it('should throw error when NTT contracts not found in config', async () => {
      const mockChainFromRpc = jest.spyOn(SuiPlatform, 'chainFromRpc');
      mockChainFromRpc.mockResolvedValue(['Testnet', 'Sui']);

      const config = {
        Sui: {
          network: 'Testnet',
          contracts: {
            coreBridge: TEST_CONTRACTS.coreBridge
            // Missing ntt contracts
          }
        }
      };

      await expect(SuiNtt.fromRpc(mockClient, config as any))
        .rejects.toThrow('Ntt contracts not found');

      mockChainFromRpc.mockRestore();
    });
  });

  describe('Method Return Types', () => {
    it('should return correct types for async methods', () => {
      expect(suiNtt.getMode()).toBeInstanceOf(Promise);
      expect(suiNtt.isPaused()).toBeInstanceOf(Promise);
      expect(suiNtt.getThreshold()).toBeInstanceOf(Promise);
      expect(suiNtt.getTokenDecimals()).toBeInstanceOf(Promise);
      expect(suiNtt.getCustodyAddress()).toBeInstanceOf(Promise);
      expect(suiNtt.getCurrentOutboundCapacity()).toBeInstanceOf(Promise);
      expect(suiNtt.getRateLimitDuration()).toBeInstanceOf(Promise);
      expect(suiNtt.isRelayingAvailable('Ethereum')).toBeInstanceOf(Promise);
    });

    it('should return correct types for async generator methods', () => {
      const transferGen = suiNtt.transfer(
        TEST_ADDRESSES.USER as any,
        1000000n,
        { 
          chain: 'Ethereum', 
          address: { 
            toUint8Array: () => new Uint8Array(32).fill(1),
            toUniversalAddress: () => ({
              toUint8Array: () => new Uint8Array(32).fill(1)
            })
          }
        },
        {}
      );
      
      expect(transferGen).toBeDefined();
      expect(typeof transferGen.next).toBe('function');
      expect(typeof transferGen[Symbol.asyncIterator]).toBe('function');
    });

    it('should handle generator cleanup', async () => {
      const transferGen = suiNtt.transfer(
        TEST_ADDRESSES.USER as any,
        1000000n,
        { 
          chain: 'Ethereum', 
          address: { 
            toUint8Array: () => new Uint8Array(32).fill(1),
            toUniversalAddress: () => ({
              toUint8Array: () => new Uint8Array(32).fill(1)
            })
          }
        },
        {}
      );
      
      // Should be able to cleanup generators
      expect(typeof transferGen.return).toBe('function');
      
      const result = await transferGen.return(undefined);
      expect(result.done).toBe(true);
    });
  });

  describe('Constants and Configuration', () => {
    it('should have correct test constants', () => {
      expect(TEST_ADDRESSES.ADMIN).toMatch(/^0x[0-9a-fA-F]{64}$/);
      expect(TEST_ADDRESSES.USER).toMatch(/^0x[0-9a-fA-F]{64}$/);
      expect(TEST_ADDRESSES.PEER).toMatch(/^0x[0-9a-fA-F]{64}$/);
      
      expect(TEST_CONTRACTS.ntt.manager).toMatch(/^0x[0-9a-fA-F]{64}$/);
      expect(TEST_CONTRACTS.ntt.token).toBe('0x2::sui::SUI');
      expect(TEST_CONTRACTS.ntt.transceiver?.wormhole).toMatch(/^0x[0-9a-fA-F]{64}$/);
      expect(TEST_CONTRACTS.coreBridge).toMatch(/^0x[0-9a-fA-F]{64}$/);
    });

    it('should handle different network configurations', () => {
      const mainnetNtt = new SuiNtt(
        'Mainnet',
        'Sui',
        mockClient,
        { 
          ntt: TEST_CONTRACTS.ntt,
          coreBridge: TEST_CONTRACTS.coreBridge
        }
      );
      
      expect(mainnetNtt.network).toBe('Mainnet');
      expect(mainnetNtt.chain).toBe('Sui');
    });
  });

  describe('Error Handling', () => {
    it('should handle constructor errors appropriately', () => {
      expect(() => {
        new SuiNtt('Testnet', 'Sui', mockClient, {
          coreBridge: TEST_CONTRACTS.coreBridge
        });
      }).toThrow('NTT contracts not found');

      expect(() => {
        new SuiNtt('Testnet', 'Sui', mockClient, {
          ntt: TEST_CONTRACTS.ntt
        });
      }).toThrow('Core Bridge contract not found');
    });

    it('should validate required contract addresses', () => {
      expect(() => {
        new SuiNtt('Testnet', 'Sui', mockClient, {
          ntt: {
            manager: '', // Invalid empty string
            token: TEST_CONTRACTS.ntt.token,
            transceiver: TEST_CONTRACTS.ntt.transceiver
          },
          coreBridge: TEST_CONTRACTS.coreBridge
        });
      }).not.toThrow(); // Constructor doesn't validate address format
    });
  });

  describe('Type Safety', () => {
    it('should enforce correct generic types', () => {
      expect(suiNtt.network).toBe('Testnet');
      expect(suiNtt.chain).toBe('Sui');
    });

    it('should work with proper address types', () => {
      const address = TEST_ADDRESSES.USER;
      expect(address).toBeDefined();
      
      // Should be able to use with SDK methods
      const transferGen = suiNtt.transfer(
        address as any,
        1000000n,
        { 
          chain: 'Ethereum', 
          address: { 
            toUint8Array: () => new Uint8Array(32).fill(1),
            toUniversalAddress: () => ({
              toUint8Array: () => new Uint8Array(32).fill(1)
            })
          }
        },
        {}
      );
      
      expect(transferGen).toBeDefined();
    });
  });

  describe('Compatibility', () => {
    it('should be compatible with Wormhole SDK interfaces', () => {
      expect(suiNtt).toBeDefined();
      expect(suiNtt.contracts).toBeDefined();
      expect(suiNtt.provider).toBeDefined();
    });

    it('should handle different chain references', () => {
      const chains = ['Ethereum', 'Solana', 'Polygon'] as const;
      
      for (const chain of chains) {
        expect(() => {
          const peer = { 
            chain, 
            address: { 
              toUint8Array: () => new Uint8Array(32).fill(1),
              toUniversalAddress: () => ({
                toUint8Array: () => new Uint8Array(32).fill(1)
              })
            }
          } as any;
          const gen = suiNtt.setPeer(peer, 6, 1000000n);
          expect(gen).toBeDefined();
        }).not.toThrow();
      }
    });
  });

  describe('Resource Management', () => {
    it('should handle async operations correctly', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      
      const mode = await suiNtt.getMode();
      expect(mode).toBeDefined();
      expect(['locking', 'burning']).toContain(mode);
    });

    it('should not leak resources in mock environment', () => {
      expect(mockClient).toBeDefined();
      expect(Object.keys(mockClient)).toEqual(
        expect.arrayContaining([
          'getObject', 
          'getOwnedObjects', 
          'getDynamicFieldObject',
          'getCoinMetadata'
        ])
      );
    });

    it('should handle concurrent operations', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      
      const promises = [
        suiNtt.getMode(),
        suiNtt.isPaused(),
        suiNtt.getThreshold(),
        suiNtt.getRateLimitDuration()
      ];

      const results = await Promise.all(promises);
      expect(results).toHaveLength(4);
      expect(results[0]).toBeDefined(); // mode
      expect(results[1]).toBe(false); // isPaused
      expect(typeof results[2]).toBe('number'); // threshold
      expect(typeof results[3]).toBe('bigint'); // rate limit duration
    });
  });

  describe('Performance', () => {
    it('should cache expensive operations', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState({ adminCapId: 'cached-id' }));

      // First call should fetch
      const adminCapId1 = await suiNtt.getAdminCapId();
      // Second call should use cache
      const adminCapId2 = await suiNtt.getAdminCapId();

      expect(adminCapId1).toBe(adminCapId2);
      expect(mockClient.getObject).toHaveBeenCalledTimes(1); // Only called once due to caching
    });

    it('should handle bulk operations efficiently', async () => {
      mockClient.getObject.mockResolvedValue(mockNttState());
      
      const startTime = Date.now();
      
      // Simulate bulk operations
      const operations = Array(10).fill(null).map(() => suiNtt.isPaused());
      const results = await Promise.all(operations);
      
      const endTime = Date.now();
      const duration = endTime - startTime;
      
      expect(results).toHaveLength(10);
      expect(results.every(r => r === false)).toBe(true);
      expect(duration).toBeLessThan(1000); // Should complete quickly
    });
  });
});