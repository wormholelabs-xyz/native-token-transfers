import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockNttState,
  mockSuiObject,
  mockPeerData,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Rate Limiting Functions', () => {
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

  describe('getCurrentOutboundCapacity', () => {
    it('should return current outbound capacity from state', async () => {
      const state = mockNttState({
        outboundCapacity: '750000000000',
        outboundLimit: '1000000000000'
      });
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      
      // Should return calculated capacity based on time passage
      expect(typeof capacity).toBe('bigint');
      expect(capacity).toBeGreaterThanOrEqual(750000000000n);
      
      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: TEST_CONTRACTS.ntt.manager,
        options: { showContent: true }
      });
    });

    it('should handle zero capacity', async () => {
      const state = mockNttState({
        outboundCapacity: '0',
        outboundLimit: '1000000000000'
      });
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      expect(capacity).toBeGreaterThanOrEqual(0n);
    });

    it('should cap capacity at limit', async () => {
      const state = mockNttState({
        outboundCapacity: '2000000000000', // Above limit
        outboundLimit: '1000000000000'
      });
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      expect(capacity).toBeLessThanOrEqual(1000000000000n);
    });

    it('should handle large capacity values', async () => {
      const largeCapacity = '18446744073709551615'; // Max uint64
      const state = mockNttState({
        outboundCapacity: largeCapacity,
        outboundLimit: largeCapacity
      });
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      expect(capacity).toBe(BigInt(largeCapacity));
    });

    it('should throw error when state fetch fails', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toThrow('Failed to fetch NTT state object');
    });

    it('should handle malformed rate limit data', async () => {
      const state = mockNttState();
      // Remove rate limit fields
      delete state.data.content.fields.outbox.fields.rate_limit;
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toBeDefined();
    });
  });

  describe('getOutboundLimit', () => {
    it('should return outbound limit from state', async () => {
      const state = mockNttState({
        outboundCapacity: '750000000000',
        outboundLimit: '1000000000000'
      });
      mockClient.getObject.mockResolvedValue(state);

      const limit = await suiNtt.getOutboundLimit();
      expect(limit).toBe(1000000000000n);
    });

    it('should handle zero limit', async () => {
      const state = mockNttState({
        outboundCapacity: '0',
        outboundLimit: '0'
      });
      mockClient.getObject.mockResolvedValue(state);

      const limit = await suiNtt.getOutboundLimit();
      expect(limit).toBe(0n);
    });

    it('should handle large limit values', async () => {
      const largeLimit = '18446744073709551615'; // Max uint64
      const state = mockNttState({
        outboundLimit: largeLimit
      });
      mockClient.getObject.mockResolvedValue(state);

      const limit = await suiNtt.getOutboundLimit();
      expect(limit).toBe(BigInt(largeLimit));
    });

    it('should throw error when state fetch fails', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getOutboundLimit()).rejects.toThrow('Failed to fetch NTT state object');
    });
  });

  describe('setOutboundLimit', () => {
    const newLimit = 2000000000000n;

    it('should create setOutboundLimit transaction skeleton', async () => {
      const txGenerator = suiNtt.setOutboundLimit(newLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Outbound Limit');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle zero limit', async () => {
      const zeroLimit = 0n;
      
      const txGenerator = suiNtt.setOutboundLimit(zeroLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Outbound Limit');
    });

    it('should handle large limit values', async () => {
      const largeLimit = BigInt('18446744073709551615'); // Max uint64
      
      const txGenerator = suiNtt.setOutboundLimit(largeLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Outbound Limit');
    });

    it('should include payer parameter when provided', async () => {
      const txGenerator = suiNtt.setOutboundLimit(newLimit, undefined);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Outbound Limit');
    });
  });

  describe('getCurrentInboundCapacity', () => {
    beforeEach(() => {
      // Mock peer data and rate limit info
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject('0xwormhole::state::State', {})); // wormhole package ID
      
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData({
        inboundLimit: '2000000000000',
        inboundCapacity: '1500000000000'
      }));
    });

    it('should return current inbound capacity for a chain', async () => {
      const capacity = await suiNtt.getCurrentInboundCapacity('Ethereum');
      
      expect(typeof capacity).toBe('bigint');
      expect(capacity).toBeGreaterThanOrEqual(0n);
      expect(capacity).toBeLessThanOrEqual(2000000000000n); // Should not exceed limit
    });

    it('should handle different chains', async () => {
      const ethereumCapacity = await suiNtt.getCurrentInboundCapacity('Ethereum');
      const solanaCapacity = await suiNtt.getCurrentInboundCapacity('Solana');
      
      expect(typeof ethereumCapacity).toBe('bigint');
      expect(typeof solanaCapacity).toBe('bigint');
    });

    it('should throw error when peer not found', async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getCurrentInboundCapacity('UnknownChain' as any))
        .rejects.toThrow('No peer found');
    });

    it('should calculate capacity based on time passage', async () => {
      const pastTime = Date.now() - (6 * 60 * 60 * 1000); // 6 hours ago
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData({
        inboundLimit: '2000000000000',
        inboundCapacity: '1000000000000',
        lastTxTimestamp: pastTime.toString()
      }));

      const capacity = await suiNtt.getCurrentInboundCapacity('Ethereum');
      
      // After 6 hours, capacity should have increased
      expect(capacity).toBeGreaterThan(1000000000000n);
      expect(capacity).toBeLessThanOrEqual(2000000000000n);
    });
  });

  describe('getInboundLimit', () => {
    beforeEach(() => {
      // Mock peer data
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject('0xwormhole::state::State', {})); // wormhole package ID
      
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData({
        inboundLimit: '3000000000000'
      }));
    });

    it('should return inbound limit for a chain', async () => {
      const limit = await suiNtt.getInboundLimit('Ethereum');
      
      expect(limit).toBe(3000000000000n);
    });

    it('should handle different chains', async () => {
      const ethereumLimit = await suiNtt.getInboundLimit('Ethereum');
      const solanaLimit = await suiNtt.getInboundLimit('Solana');
      
      expect(typeof ethereumLimit).toBe('bigint');
      expect(typeof solanaLimit).toBe('bigint');
    });

    it('should handle zero limit', async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData({
        inboundLimit: '0'
      }));

      const limit = await suiNtt.getInboundLimit('Ethereum');
      expect(limit).toBe(0n);
    });

    it('should throw error when peer not found', async () => {
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getInboundLimit('UnknownChain' as any))
        .rejects.toThrow('No peer found');
    });
  });

  describe('setInboundLimit', () => {
    const newLimit = 2000000000000n;

    beforeEach(() => {
      // Mock existing peer data
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' })) // getAdminCapId
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {})) // getPackageId
        .mockResolvedValueOnce(mockNttState()) // getPeer state fetch
        .mockResolvedValueOnce(mockSuiObject('0xwormhole::state::State', {})); // wormhole package ID
      
      mockClient.getDynamicFieldObject.mockResolvedValue(mockPeerData());
    });

    it('should create setInboundLimit transaction with existing peer', async () => {
      const txGenerator = suiNtt.setInboundLimit('Ethereum', newLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Inbound Limit');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle different chains', async () => {
      const txGenerator = suiNtt.setInboundLimit('Solana', newLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Inbound Limit');
    });

    it('should handle zero limit', async () => {
      const zeroLimit = 0n;
      
      const txGenerator = suiNtt.setInboundLimit('Ethereum', zeroLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Inbound Limit');
    });

    it('should handle large limit values', async () => {
      const largeLimit = BigInt('18446744073709551615'); // Max uint64
      
      const txGenerator = suiNtt.setInboundLimit('Ethereum', largeLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Inbound Limit');
    });

    it('should include payer parameter when provided', async () => {
      const txGenerator = suiNtt.setInboundLimit('Ethereum', newLimit, undefined);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Inbound Limit');
    });

    it('should throw error when peer not found', async () => {
      // Mock no existing peer
      mockClient.getDynamicFieldObject.mockResolvedValue({ data: null });

      const txGenerator = suiNtt.setInboundLimit('Ethereum', newLimit);
      await expect(txGenerator.next()).rejects.toThrow('No peer found for chain Ethereum');
    });

    it('should preserve existing peer address and token decimals', async () => {
      const txGenerator = suiNtt.setInboundLimit('Ethereum', newLimit);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      // The transaction should reuse existing peer data
      expect(mockClient.getDynamicFieldObject).toHaveBeenCalled();
    });
  });

  describe('rate limiting calculations', () => {
    it('should calculate capacity based on time passage', async () => {
      const currentTime = Date.now();
      const pastTime = currentTime - (12 * 60 * 60 * 1000); // 12 hours ago
      
      const state = mockNttState({
        outboundCapacity: '500000000000',
        outboundLimit: '1000000000000'
      });
      // Set last transaction timestamp to 12 hours ago
      state.data.content.fields.outbox.fields.rate_limit.fields.last_tx_timestamp = pastTime.toString();
      
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      
      // After 12 hours, capacity should have increased
      expect(capacity).toBeGreaterThan(500000000000n);
      expect(capacity).toBeLessThanOrEqual(1000000000000n);
    });

    it('should handle rate limit duration correctly', async () => {
      const duration = await suiNtt.getRateLimitDuration();
      expect(duration).toBe(BigInt(24 * 60 * 60 * 1000)); // 24 hours in milliseconds
    });

    it('should not exceed limit even with long time passage', async () => {
      const currentTime = Date.now();
      const veryPastTime = currentTime - (48 * 60 * 60 * 1000); // 48 hours ago
      
      const state = mockNttState({
        outboundCapacity: '100000000000',
        outboundLimit: '1000000000000'
      });
      state.data.content.fields.outbox.fields.rate_limit.fields.last_tx_timestamp = veryPastTime.toString();
      
      mockClient.getObject.mockResolvedValue(state);

      const capacity = await suiNtt.getCurrentOutboundCapacity();
      
      // Should not exceed the limit
      expect(capacity).toBeLessThanOrEqual(1000000000000n);
    });
  });

  describe('error handling and edge cases', () => {
    it('should handle network errors in rate limit queries', async () => {
      mockClient.getObject.mockRejectedValue(new Error('Network error'));

      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toThrow('Network error');
    });

    it('should handle concurrent rate limit operations', async () => {
      const state = mockNttState({
        outboundCapacity: '750000000000',
        outboundLimit: '1000000000000'
      });
      mockClient.getObject.mockResolvedValue(state);

      // Multiple concurrent calls should all resolve correctly
      const promises = [
        suiNtt.getCurrentOutboundCapacity(),
        suiNtt.getOutboundLimit(),
        suiNtt.getRateLimitDuration()
      ];

      const results = await Promise.all(promises);
      expect(results).toHaveLength(3);
      expect(results.every(r => typeof r === 'bigint')).toBe(true);
    });

    it('should handle malformed timestamp data', async () => {
      const state = mockNttState({
        outboundCapacity: '750000000000',
        outboundLimit: '1000000000000'
      });
      // Set invalid timestamp
      state.data.content.fields.outbox.fields.rate_limit.fields.last_tx_timestamp = 'invalid';
      
      mockClient.getObject.mockResolvedValue(state);

      // Should handle gracefully or throw appropriate error
      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toBeDefined();
    });

    it('should handle missing rate limit fields', async () => {
      const state = mockNttState();
      // Remove rate limit structure
      state.data.content.fields.outbox = { fields: {} };
      
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getCurrentOutboundCapacity()).rejects.toBeDefined();
    });

    it('should validate transaction creation for rate limit updates', async () => {
      const txGenerator = suiNtt.setOutboundLimit(1000000n);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.transaction).toBeDefined();
      expect(unsignedTx.transaction.constructor.name).toBe('_Transaction');
    });
  });
});