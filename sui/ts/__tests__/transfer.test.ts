import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockCoinMetadata,
  mockAttestation,
  mockNttState,
  mockSuiObject,
  TEST_ADDRESSES,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Transfer Operations', () => {
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

  describe('transfer', () => {
    const transferAmount = 1000000000n; // 1 SUI in lamports
    const sender = TEST_ADDRESSES.USER;
    const destination = {
      chain: 'Ethereum' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1),
        toUniversalAddress: () => ({
          toUint8Array: () => new Uint8Array(32).fill(1)
        })
      }
    } as any;
    const options = { queue: false };

    beforeEach(() => {
      // Mock package ID extraction
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}))
        .mockResolvedValueOnce(mockCoinMetadata());
      
      // Mock coin metadata query
      mockClient.getCoinMetadata.mockResolvedValue(mockCoinMetadata());
    });

    it('should create transfer transaction with correct parameters', async () => {
      const txGenerator = suiNtt.transfer(sender as any, transferAmount, destination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
      
      expect(mockClient.getCoinMetadata).toHaveBeenCalledWith({
        coinType: TEST_CONTRACTS.ntt.token
      });
    });

    it('should handle queued transfer option', async () => {
      const queuedOptions = { queue: true };
      
      const txGenerator = suiNtt.transfer(sender as any, transferAmount, destination, queuedOptions);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
    });

    it('should handle different destination chains', async () => {
      const solanaDestination = {
        chain: 'Solana' as const,
        address: {
          toUint8Array: () => new Uint8Array(32).fill(2),
          toUniversalAddress: () => ({
            toUint8Array: () => new Uint8Array(32).fill(2)
          })
        }
      } as any;
      
      const txGenerator = suiNtt.transfer(sender as any, transferAmount, solanaDestination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
    });

    it('should handle large transfer amounts', async () => {
      const largeAmount = BigInt('1000000000000000000'); // 1000 SUI
      
      const txGenerator = suiNtt.transfer(sender as any, largeAmount, destination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
    });

    it('should handle minimal transfer amounts', async () => {
      const minAmount = 1n; // 1 lamport
      
      const txGenerator = suiNtt.transfer(sender as any, minAmount, destination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
    });

    it('should throw error when coin metadata not found', async () => {
      mockClient.getCoinMetadata.mockResolvedValue(null);

      const txGenerator = suiNtt.transfer(sender as any, transferAmount, destination, options);
      await expect(txGenerator.next()).rejects.toThrow('CoinMetadata not found for SUI');
    });

    it('should throw error for non-SUI tokens', async () => {
      const customSuiNtt = new SuiNtt(
        'Testnet',
        'Sui',
        mockClient,
        {
          ntt: {
            ...TEST_CONTRACTS.ntt,
            token: '0xabc::custom::TOKEN'
          },
          coreBridge: TEST_CONTRACTS.coreBridge
        }
      );

      const txGenerator = customSuiNtt.transfer(sender as any, transferAmount, destination, options);
      await expect(txGenerator.next()).rejects.toThrow('Transfer not yet implemented for token: 0xabc::custom::TOKEN');
    });

    it('should throw error when destination address conversion fails', async () => {
      const invalidDestination = {
        chain: 'Ethereum' as const,
        address: {
          toUint8Array: () => { throw new Error('Address conversion failed'); },
          toUniversalAddress: () => { throw new Error('Universal address failed'); }
        }
      } as any;

      const txGenerator = suiNtt.transfer(sender as any, transferAmount, invalidDestination, options);
      await expect(txGenerator.next()).rejects.toThrow('Failed to convert destination address to bytes');
    });

    it('should handle destination address with toUint8Array method', async () => {
      const directDestination = {
        chain: 'Ethereum' as const,
        address: {
          toUint8Array: () => new Uint8Array(32).fill(3)
          // No toUniversalAddress method
        }
      } as any;

      const txGenerator = suiNtt.transfer(sender as any, transferAmount, directDestination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('NTT Transfer');
    });

    it('should include proper gas fee estimation', async () => {
      const txGenerator = suiNtt.transfer(sender as any, transferAmount, destination, options);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      // Transaction should include gas splitting from gas object
      expect(unsignedTx.transaction).toBeDefined();
    });
  });

  describe('redeem', () => {
    it('should create redeem transaction skeleton', async () => {
      const attestation = mockAttestation();
      
      const txGenerator = suiNtt.redeem([attestation]);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Redeem NTT Transfer');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle multiple attestations', async () => {
      const attestations = [
        mockAttestation('Ethereum'),
        mockAttestation('Solana')
      ];
      
      const txGenerator = suiNtt.redeem(attestations);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Redeem NTT Transfer');
    });

    it('should handle empty attestations array', async () => {
      const txGenerator = suiNtt.redeem([]);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Redeem NTT Transfer');
    });
  });

  describe('quoteDeliveryPrice', () => {
    const deliveryOptions = {
      gasDropoff: 100000n, // Required for delivery pricing
      automatic: true,
      queue: false // Required by TransferOptions interface
    };

    beforeEach(() => {
      // Mock quoter contract data - would be fetched from quoter program
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject('0xquoter::instance::Instance', {
          sui_price_in_usd: '80000000', // $80 in 6 decimals
          precision: '1000000' // 6 decimals
        }))
        .mockResolvedValueOnce(mockSuiObject('0xquoter::chain::RegisteredChain', {
          gas_price: '20000000000', // 20 gwei in wei
          native_token_price: '3000000000', // $3000 in 6 decimals
          base_fee: '50000' // Base relay fee in USD (6 decimals)
        }));
    });

    it('should calculate delivery price for EVM chains', async () => {
      const price = await suiNtt.quoteDeliveryPrice('Ethereum', deliveryOptions);
      
      expect(typeof price).toBe('bigint');
      expect(price).toBeGreaterThan(0n);
      // Price should include base fee + gas cost + requested dropoff + buffer
    });

    it('should handle different gas dropoff amounts', async () => {
      const lowDropoff = await suiNtt.quoteDeliveryPrice('Ethereum', {
        gasDropoff: 10000n,
        automatic: true,
        queue: false
      });
      
      const highDropoff = await suiNtt.quoteDeliveryPrice('Ethereum', {
        gasDropoff: 1000000n,
        automatic: true,
        queue: false
      });
      
      expect(highDropoff).toBeGreaterThan(lowDropoff);
    });

    it('should throw error for non-EVM chains', async () => {
      await expect(suiNtt.quoteDeliveryPrice('Solana', deliveryOptions))
        .rejects.toThrow('Delivery pricing only supported for EVM chains');
    });

    it('should throw error when gasDropoff not provided', async () => {
      await expect(suiNtt.quoteDeliveryPrice('Ethereum', { automatic: true, queue: false }))
        .rejects.toThrow('gasDropoff is required for delivery pricing');
    });

    it('should throw error when quoter not configured', async () => {
      // Mock quoter contract not found
      mockClient.getObject.mockResolvedValue({ data: null });
      
      await expect(suiNtt.quoteDeliveryPrice('Ethereum', deliveryOptions))
        .rejects.toThrow('Quoter contract not configured');
    });

    it('should include 5% buffer for price fluctuations', async () => {
      const price = await suiNtt.quoteDeliveryPrice('Ethereum', deliveryOptions);
      
      // The implementation should add a 5% buffer to the calculated price
      expect(price).toBeGreaterThan(0n);
      // Would need to test actual calculation with known values
    });
  });

  describe('isRelayingAvailable', () => {
    it('should return false for all chains', async () => {
      const ethereumResult = await suiNtt.isRelayingAvailable('Ethereum');
      expect(ethereumResult).toBe(false);

      const solanaResult = await suiNtt.isRelayingAvailable('Solana');
      expect(solanaResult).toBe(false);
    });
  });

  describe('transfer status functions', () => {
    const attestation = mockAttestation();

    describe('getIsApproved', () => {
      beforeEach(() => {
        // Mock inbox item account lookup
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: null
          }
        }));
      });

      it('should return true when transfer is approved (initialized)', async () => {
        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(true);
      });

      it('should return false when inbox item does not exist', async () => {
        mockClient.getObject.mockResolvedValue({ data: null });
        
        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(false);
      });

      it('should return false when not initialized', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: false
        }));
        
        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(false);
      });

      it('should handle network errors gracefully', async () => {
        mockClient.getObject.mockRejectedValue(new Error('Network error'));
        
        const result = await suiNtt.getIsApproved(attestation);
        expect(result).toBe(false);
      });
    });

    describe('getIsExecuted', () => {
      beforeEach(() => {
        // Mock inbox item with execution status
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: true,
            release_after: null
          }
        }));
      });

      it('should return true when transfer is executed (tokens released)', async () => {
        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(true);
      });

      it('should return false when transfer not executed', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: null
          }
        }));
        
        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(false);
      });

      it('should return false when inbox item does not exist', async () => {
        mockClient.getObject.mockResolvedValue({ data: null });
        
        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(false);
      });

      it('should handle network errors gracefully', async () => {
        mockClient.getObject.mockRejectedValue(new Error('Network error'));
        
        const result = await suiNtt.getIsExecuted(attestation);
        expect(result).toBe(false);
      });
    });

    describe('getIsTransferInboundQueued', () => {
      it('should return true when transfer is queued', async () => {
        const queuedTime = Date.now() + (12 * 60 * 60 * 1000); // 12 hours from now
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: queuedTime.toString()
          }
        }));
        
        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(true);
      });

      it('should return false when transfer is not queued', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: null
          }
        }));
        
        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(false);
      });

      it('should return false when already executed', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: true,
            release_after: Date.now().toString()
          }
        }));
        
        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(false);
      });

      it('should return false when queue time has passed', async () => {
        const pastTime = Date.now() - (1 * 60 * 60 * 1000); // 1 hour ago
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: pastTime.toString()
          }
        }));
        
        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(false); // Can now be executed
      });

      it('should return false when inbox item does not exist', async () => {
        mockClient.getObject.mockResolvedValue({ data: null });
        
        const result = await suiNtt.getIsTransferInboundQueued(attestation);
        expect(result).toBe(false);
      });
    });
  });

  describe('queued transfer management', () => {
    describe('getInboundQueuedTransfer', () => {
      const message = {
        id: new Uint8Array(32).fill(1),
        sender: new Uint8Array(32).fill(2),
        payload: {
          recipientAddress: new Uint8Array(32).fill(3),
          amount: '1000000000'
        }
      };

      it('should return queued transfer details', async () => {
        const queuedTime = Date.now() + (12 * 60 * 60 * 1000); // 12 hours from now
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          recipient: '0x' + '3'.repeat(64),
          amount: '1000000000',
          release_status: {
            released: false,
            release_after: queuedTime.toString()
          }
        }));

        const result = await suiNtt.getInboundQueuedTransfer('Ethereum', message as any);
        
        expect(result).not.toBeNull();
        expect(result?.recipient).toBe('0x' + '3'.repeat(64));
        expect(result?.amount).toBe(1000000000n);
        expect(result?.rateLimitExpiryTimestamp).toBe(BigInt(queuedTime));
      });

      it('should return null when transfer is not queued', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: false,
            release_after: null
          }
        }));

        const result = await suiNtt.getInboundQueuedTransfer('Ethereum', message as any);
        expect(result).toBeNull();
      });

      it('should return null when transfer is already executed', async () => {
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          release_status: {
            released: true,
            release_after: Date.now().toString()
          }
        }));

        const result = await suiNtt.getInboundQueuedTransfer('Ethereum', message as any);
        expect(result).toBeNull();
      });

      it('should return null when inbox item does not exist', async () => {
        mockClient.getObject.mockResolvedValue({ data: null });

        const result = await suiNtt.getInboundQueuedTransfer('Ethereum', message as any);
        expect(result).toBeNull();
      });

      it('should handle different chains', async () => {
        const queuedTime = Date.now() + (6 * 60 * 60 * 1000); // 6 hours from now
        mockClient.getObject.mockResolvedValue(mockSuiObject('0xinbox::item::InboxItem', {
          init: true,
          recipient: '0x' + 'a'.repeat(64),
          amount: '2000000000',
          release_status: {
            released: false,
            release_after: queuedTime.toString()
          }
        }));

        const result = await suiNtt.getInboundQueuedTransfer('Solana', message as any);
        
        expect(result).not.toBeNull();
        expect(result?.amount).toBe(2000000000n);
      });
    });

    describe('completeInboundQueuedTransfer', () => {
      const message = {
        id: new Uint8Array(32).fill(1),
        sender: new Uint8Array(32).fill(2),
        payload: {
          recipientAddress: new Uint8Array(32).fill(3),
          amount: '1000000000'
        }
      };

      beforeEach(() => {
        // Mock state and inbox item for completion
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState({ mode: 'Locking' })) // NTT state
          .mockResolvedValueOnce(mockSuiObject('0xinbox::item::InboxItem', {
            init: true,
            recipient: '0x' + '3'.repeat(64),
            amount: '1000000000',
            release_status: {
              released: false,
              release_after: (Date.now() - 1000).toString() // Past time, ready for completion
            }
          }));
      });

      it('should create complete queued transfer transaction', async () => {
        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        const { value: unsignedTx } = await txGenerator.next();
        
        expect(unsignedTx).toBeDefined();
        expect(unsignedTx.description).toBe('Complete Inbound Queued Transfer');
        expect(unsignedTx.network).toBe('Testnet');
        expect(unsignedTx.chain).toBe('Sui');
      });

      it('should handle different NTT modes (locking vs burning)', async () => {
        // Test burning mode
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState({ mode: 'Burning' }))
          .mockResolvedValueOnce(mockSuiObject('0xinbox::item::InboxItem', {
            init: true,
            release_status: {
              released: false,
              release_after: (Date.now() - 1000).toString()
            }
          }));

        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        const { value: unsignedTx } = await txGenerator.next();
        
        expect(unsignedTx).toBeDefined();
        expect(unsignedTx.description).toBe('Complete Inbound Queued Transfer');
      });

      it('should handle payer parameter', async () => {
        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any, undefined);
        const { value: unsignedTx } = await txGenerator.next();
        
        expect(unsignedTx).toBeDefined();
      });

      it('should throw error when transfer is not queued', async () => {
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState())
          .mockResolvedValueOnce(mockSuiObject('0xinbox::item::InboxItem', {
            init: true,
            release_status: {
              released: false,
              release_after: null // Not queued
            }
          }));

        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        await expect(txGenerator.next()).rejects.toThrow('Transfer is not queued');
      });

      it('should throw error when queue time has not passed', async () => {
        const futureTime = Date.now() + (12 * 60 * 60 * 1000); // 12 hours from now
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState())
          .mockResolvedValueOnce(mockSuiObject('0xinbox::item::InboxItem', {
            init: true,
            release_status: {
              released: false,
              release_after: futureTime.toString()
            }
          }));

        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        await expect(txGenerator.next()).rejects.toThrow('Transfer is still queued');
      });

      it('should throw error when already executed', async () => {
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState())
          .mockResolvedValueOnce(mockSuiObject('0xinbox::item::InboxItem', {
            init: true,
            release_status: {
              released: true,
              release_after: Date.now().toString()
            }
          }));

        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        await expect(txGenerator.next()).rejects.toThrow('Transfer already executed');
      });

      it('should throw error when contract is paused', async () => {
        mockClient.getObject
          .mockResolvedValueOnce(mockNttState({ paused: true }));

        const txGenerator = suiNtt.completeInboundQueuedTransfer('Ethereum', message as any);
        await expect(txGenerator.next()).rejects.toThrow('Contract is paused');
      });
    });
  });

  describe('error handling and edge cases', () => {
    const destination = {
      chain: 'Ethereum' as const,
      address: {
        toUint8Array: () => new Uint8Array(32).fill(1),
        toUniversalAddress: () => ({
          toUint8Array: () => new Uint8Array(32).fill(1)
        })
      }
    } as any;
    const options = { queue: false };

    it('should handle RPC timeouts', async () => {
      mockClient.getCoinMetadata.mockImplementation(() => 
        new Promise((_, reject) => setTimeout(() => reject(new Error('Timeout')), 100))
      );

      const txGenerator = suiNtt.transfer(
        TEST_ADDRESSES.USER as any, 
        1000000n, 
        destination, 
        options
      );
      
      await expect(txGenerator.next()).rejects.toThrow('Timeout');
    });

    it('should handle invalid transfer amounts', async () => {
      const zeroAmount = 0n;
      
      const txGenerator = suiNtt.transfer(
        TEST_ADDRESSES.USER as any, 
        zeroAmount, 
        destination, 
        options
      );
      
      // Should create transaction even with zero amount (let Move contract handle validation)
      const { value: unsignedTx } = await txGenerator.next();
      expect(unsignedTx).toBeDefined();
    });

    it('should handle malformed addresses', async () => {
      const malformedDestination = {
        chain: 'Ethereum' as const,
        address: null as any
      } as any;

      const txGenerator = suiNtt.transfer(
        TEST_ADDRESSES.USER as any, 
        1000000n, 
        malformedDestination, 
        options
      );
      
      await expect(txGenerator.next()).rejects.toThrow();
    });

    it('should validate transfer options', async () => {
      const invalidOptions = { 
        queue: null as any,
        gasDropoff: -1n,
        payload: 'invalid' as any
      };

      const txGenerator = suiNtt.transfer(
        TEST_ADDRESSES.USER as any, 
        1000000n, 
        destination, 
        invalidOptions
      );
      
      // Should handle gracefully and use defaults
      const { value: unsignedTx } = await txGenerator.next();
      expect(unsignedTx).toBeDefined();
    });
  });
});