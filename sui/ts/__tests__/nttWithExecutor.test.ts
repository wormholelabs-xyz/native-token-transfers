import { SuiNttWithExecutor } from '../src/nttWithExecutor';
import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockCoinMetadata,
  mockSuiObject,
  TEST_ADDRESSES,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNttWithExecutor', () => {
  let suiNttWithExecutor: SuiNttWithExecutor<'Testnet', 'Sui'>;
  let suiNtt: SuiNtt<'Testnet', 'Sui'>;
  let mockClient: jest.Mocked<any>;


  const mockQuote = {
    signedQuote: new Uint8Array([1, 2, 3, 4]),
    relayInstructions: new Uint8Array([5, 6, 7, 8]),
    estimatedCost: 1000000n, // 0.001 SUI
    payeeAddress: new Uint8Array(32).fill(0xaa),
    referrer: {
      chain: 'Solana' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(0xbb)
      }
    } as any,
    referrerFee: 500000n, // 0.0005 SUI
    remainingAmount: 999500000n, // 0.9995 SUI
    referrerFeeDbps: 5n, // 0.05%
    expires: new Date(Date.now() + 3600000), // 1 hour from now
    gasDropOff: 100000n, // 0.0001 SUI
  };

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

    suiNttWithExecutor = new SuiNttWithExecutor(
      'Testnet',
      'Sui',
      mockClient,
      { 
        ntt: TEST_CONTRACTS.ntt,
        coreBridge: TEST_CONTRACTS.coreBridge
      }
    );

    // Setup common mocks
    mockClient.getObject
      .mockResolvedValueOnce(mockSuiObject('0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456::ntt::State', {})) // getPackageId
      .mockResolvedValueOnce({ // getWormholePackageId object fields
        data: {
          content: {
            dataType: 'moveObject',
            fields: {
              value: {
                fields: {
                  package: '0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456'
                }
              }
            }
          }
        }
      })
      .mockResolvedValue(mockCoinMetadata()); // getCoinMetadata - use mockResolvedValue to reuse for multiple calls
    
    mockClient.getCoinMetadata
      .mockResolvedValue(mockCoinMetadata()); // Mock getCoinMetadata method

    // Mock getDynamicFields for getWormholePackageId
    mockClient.getDynamicFields
      .mockResolvedValue({
        data: [{
          name: { type: 'CurrentPackage' },
          objectId: '0x1234567890abcdef1234567890abcdef12345678901234567890abcdef123456'
        }],
        hasNextPage: false,
        nextCursor: null
      });

    // Mock SuiPlatform.getCoins for non-native token handling
    const mockGetCoins = jest.spyOn(require('@wormhole-foundation/sdk-sui').SuiPlatform, 'getCoins');
    mockGetCoins.mockResolvedValue([
      {
        coinObjectId: '0xcoin1234567890abcdef1234567890abcdef12345678901234567890abcdef',
        coinType: '0x2::sui::SUI',
        balance: 1000000000n
      }
    ]);
  });

  describe('constructor', () => {
    it('should initialize with correct configuration', () => {
      expect(suiNttWithExecutor.network).toBe('Testnet');
      expect(suiNttWithExecutor.chain).toBe('Sui');
      expect(suiNttWithExecutor.executorPackageId).toBeDefined();
      expect(suiNttWithExecutor.executorStateId).toBeDefined();
      expect(typeof suiNttWithExecutor.executorPackageId).toBe('string');
      expect(typeof suiNttWithExecutor.executorStateId).toBe('string');
    });
  });

  describe('transfer', () => {
    const sender = TEST_ADDRESSES.USER as any;
    const destination = {
      chain: 'Solana' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1),
        toUniversalAddress: () => ({
          toUint8Array: () => new Uint8Array(32).fill(1)
        })
      }
    } as any;
    const amount = 1000000000n; // 1 SUI

    beforeEach(() => {
      // The new implementation creates its own transaction and doesn't use ntt.transfer()
      // So we don't need to mock ntt.transfer anymore
    });

    it('should generate executor-enhanced transfer transaction', async () => {
      const txGenerator = suiNttWithExecutor.transfer(
        sender,
        destination,
        amount,
        mockQuote,
        suiNtt
      );

      const { value: tx } = await txGenerator.next();
      
      expect(tx).toBeDefined();
      expect(tx.description).toBe('NTT Transfer with Executor');
      expect(tx.network).toBe('Testnet');
      expect(tx.chain).toBe('Sui');
      
      // Verify that we get a transaction (the actual implementation creates its own transaction)
      expect(tx.transaction).toBeDefined();
    });

    it('should throw error for expired quote', async () => {
      const expiredQuote = {
        ...mockQuote,
        expires: new Date(Date.now() - 3600000) // 1 hour ago
      };

      const txGenerator = suiNttWithExecutor.transfer(
        sender,
        destination,
        amount,
        expiredQuote,
        suiNtt
      );

      await expect(txGenerator.next()).rejects.toThrow('Quote has expired');
    });

    it('should throw error for non-Solana destination chains', async () => {
      const ethereumDestination = {
        chain: 'Ethereum' as const,
        address: {
          toUint8Array: () => new Uint8Array(32).fill(2)
        }
      } as any;

      const txGenerator = suiNttWithExecutor.transfer(
        sender,
        ethereumDestination,
        amount,
        mockQuote,
        suiNtt
      );

      await expect(txGenerator.next()).rejects.toThrow('Executor only supports Solana destination chains');
    });

    it('should handle zero referrer fee', async () => {
      const quoteWithoutReferrerFee = {
        ...mockQuote,
        referrerFee: 0n,
        remainingAmount: amount
      };

      const txGenerator = suiNttWithExecutor.transfer(
        sender,
        destination,
        amount,
        quoteWithoutReferrerFee,
        suiNtt
      );

      const { value: tx } = await txGenerator.next();
      
      expect(tx).toBeDefined();
      expect(tx.description).toBe('NTT Transfer with Executor');
    });

    it('should create single combined NTT + executor transaction', async () => {
      const txGenerator = suiNttWithExecutor.transfer(
        sender,
        destination,
        amount,
        mockQuote,
        suiNtt
      );

      // Should generate a single transaction that combines NTT transfer and executor logic
      const { value: tx } = await txGenerator.next();
      expect(tx.description).toBe('NTT Transfer with Executor');

      // Should be the only transaction generated
      const { done } = await txGenerator.next();
      expect(done).toBe(true);
    });
  });

  describe('estimateMsgValueAndGasLimit', () => {
    it('should estimate costs for transfer without recipient', async () => {
      const estimate = await suiNttWithExecutor.estimateMsgValueAndGasLimit(undefined);
      
      expect(estimate.msgValue).toBeGreaterThan(0n);
      expect(estimate.gasLimit).toBeGreaterThan(0n);
      expect(typeof estimate.msgValue).toBe('bigint');
      expect(typeof estimate.gasLimit).toBe('bigint');
    });

    it('should estimate higher costs when recipient is provided', async () => {
      const recipient = {
        chain: 'Ethereum' as const,
        address: {
          toUint8Array: () => new Uint8Array(32).fill(3)
        }
      } as any;

      const withoutRecipient = await suiNttWithExecutor.estimateMsgValueAndGasLimit(undefined);
      const withRecipient = await suiNttWithExecutor.estimateMsgValueAndGasLimit(recipient);
      
      expect(withRecipient.msgValue).toBeGreaterThan(withoutRecipient.msgValue);
      expect(withRecipient.gasLimit).toBe(withoutRecipient.gasLimit); // Gas limit should be the same
    });

    it('should include 10% buffer in cost estimation', async () => {
      const estimate = await suiNttWithExecutor.estimateMsgValueAndGasLimit(undefined);
      
      // Base costs before buffer: 1M + 3M + 2M + 0.5M = 6.5M
      const baseCost = 6_500_000n;
      const expectedWithBuffer = (baseCost * 110n) / 100n; // 7.15M
      
      expect(estimate.msgValue).toBe(expectedWithBuffer);
    });
  });

  describe('validateExecutorConfig', () => {
    beforeEach(() => {
      // Reset the mock for each test in this describe block
      mockClient.getObject.mockReset();
    });

    it('should return true for valid executor configuration', async () => {
      mockClient.getObject.mockResolvedValue({
        data: {
          content: {
            dataType: 'moveObject',
            type: '0xexecutor::state::ExecutorState',
            fields: {}
          }
        }
      });

      const isValid = await suiNttWithExecutor.validateExecutorConfig();
      expect(isValid).toBe(true);
    });

    it('should return false for invalid executor configuration', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      const isValid = await suiNttWithExecutor.validateExecutorConfig();
      expect(isValid).toBe(false);
    });

    it('should handle RPC errors gracefully', async () => {
      mockClient.getObject.mockRejectedValue(new Error('RPC error'));

      const isValid = await suiNttWithExecutor.validateExecutorConfig();
      expect(isValid).toBe(false);
    });
  });

  describe('getSupportedDestinationChains', () => {
    it('should return supported Solana chain', async () => {
      const supportedChains = await suiNttWithExecutor.getSupportedDestinationChains();
      
      expect(supportedChains).toContain('Solana');
      expect(supportedChains).not.toContain('Ethereum'); // Not Solana
    });

    it('should return only Solana chain', async () => {
      const supportedChains = await suiNttWithExecutor.getSupportedDestinationChains();
      
      expect(supportedChains).toEqual(['Solana']);
    });
  });

  describe('fromRpc factory method', () => {
    it('should create instance from RPC configuration', async () => {
      // Mock the platform detection
      const mockChainFromRpc = jest.spyOn(
        require('@wormhole-foundation/sdk-sui').SuiPlatform,
        'chainFromRpc'
      );
      mockChainFromRpc.mockResolvedValue(['Testnet', 'Sui']);

      const config = {
        Sui: {
          network: 'Testnet' as const,
          contracts: {
            ntt: TEST_CONTRACTS.ntt,
            coreBridge: TEST_CONTRACTS.coreBridge
          },
          // Add required ChainConfig fields
          key: 'Sui',
          platform: 'Sui',
          chainId: 21,
          finalityThreshold: 1,
          nativeTokenDecimals: 9,
          nativeTokenSymbol: 'SUI',
          rpc: 'https://testnet.sui.io',
        } as any
      };

      const instance = await SuiNttWithExecutor.fromRpc(mockClient, config);
      
      expect(instance).toBeInstanceOf(SuiNttWithExecutor);
      expect(instance.network).toBe('Testnet');
      expect(instance.chain).toBe('Sui');
      expect(instance.executorPackageId).toBeDefined();

      mockChainFromRpc.mockRestore();
    });

    it('should throw error for network mismatch', async () => {
      const mockChainFromRpc = jest.spyOn(
        require('@wormhole-foundation/sdk-sui').SuiPlatform,
        'chainFromRpc'
      );
      mockChainFromRpc.mockResolvedValue(['Mainnet', 'Sui']);

      const config = {
        Sui: {
          network: 'Testnet' as const, // Mismatch with detected 'Mainnet'
          contracts: {
            ntt: TEST_CONTRACTS.ntt,
            coreBridge: TEST_CONTRACTS.coreBridge
          },
          // Add required ChainConfig fields
          key: 'Sui',
          platform: 'Sui',
          chainId: 21,
          finalityThreshold: 1,
          nativeTokenDecimals: 9,
          nativeTokenSymbol: 'SUI',
          rpc: 'https://testnet.sui.io',
        } as any
      };

      await expect(SuiNttWithExecutor.fromRpc(mockClient, config))
        .rejects.toThrow('Network mismatch: Testnet != Mainnet');

      mockChainFromRpc.mockRestore();
    });
  });
});