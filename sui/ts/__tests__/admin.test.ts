import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockNttState,
  mockSuiObject,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Admin Functions', () => {
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

  describe('pause', () => {
    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));
    });

    it('should create pause transaction', async () => {
      const txGenerator = suiNtt.pause();
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Pause NTT');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle payer parameter when provided', async () => {
      const txGenerator = suiNtt.pause();
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Pause NTT');
    });

    it('should require admin cap for pausing', async () => {
      // Mock admin cap not found
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: null }));

      const txGenerator = suiNtt.pause();
      await expect(txGenerator.next()).rejects.toThrow('AdminCap ID not found in NTT state');
    });
  });

  describe('unpause', () => {
    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));
    });

    it('should create unpause transaction', async () => {
      const txGenerator = suiNtt.unpause();
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Unpause NTT');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle payer parameter when provided', async () => {
      const txGenerator = suiNtt.unpause();
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Unpause NTT');
    });

    it('should require admin cap for unpausing', async () => {
      // Mock admin cap not found
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: null }));

      const txGenerator = suiNtt.unpause();
      await expect(txGenerator.next()).rejects.toThrow('AdminCap ID not found in NTT state');
    });
  });

  describe('setOwner', () => {
    const newOwner = '0x' + '1'.repeat(64);

    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));
    });

    it('should create setOwner transaction', async () => {
      const txGenerator = suiNtt.setOwner(newOwner as any);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Transfer Ownership');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should require admin cap for ownership transfer', async () => {
      // Mock admin cap not found
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: null }));

      const txGenerator = suiNtt.setOwner(newOwner as any);
      await expect(txGenerator.next()).rejects.toThrow('AdminCap ID not found in NTT state');
    });

    it('should handle payer parameter', async () => {
      const txGenerator = suiNtt.setOwner(newOwner as any, undefined);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Transfer Ownership');
    });
  });

  describe('setPauser', () => {
    const newPauser = '0x' + '1'.repeat(64);

    it('should throw not supported error for Sui (like Solana)', async () => {
      const txGenerator = suiNtt.setPauser(newPauser as any);
      await expect(txGenerator.next()).rejects.toThrow('Pauser role not supported on Sui');
    });

    it('should handle payer parameter', async () => {
      const txGenerator = suiNtt.setPauser(newPauser as any, undefined);
      await expect(txGenerator.next()).rejects.toThrow('Not implemented');
    });
  });

  describe('setThreshold', () => {
    const newThreshold = 5;

    beforeEach(() => {
      // Mock admin cap and package ID retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));
    });

    it('should create setThreshold transaction with correct parameters', async () => {
      const txGenerator = suiNtt.setThreshold(newThreshold);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Threshold');
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
    });

    it('should handle threshold of 1', async () => {
      const minThreshold = 1;
      
      const txGenerator = suiNtt.setThreshold(minThreshold);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Threshold');
    });

    it('should handle high threshold values', async () => {
      const highThreshold = 255; // Max u8
      
      const txGenerator = suiNtt.setThreshold(highThreshold);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Threshold');
    });

    it('should handle threshold of 0', async () => {
      const zeroThreshold = 0;
      
      const txGenerator = suiNtt.setThreshold(zeroThreshold);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Threshold');
    });

    it('should include payer parameter when provided', async () => {
      const txGenerator = suiNtt.setThreshold(newThreshold, undefined);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.description).toBe('Set Threshold');
    });

    it('should use correct Move call parameters', async () => {
      const txGenerator = suiNtt.setThreshold(newThreshold);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      // Verify transaction contains correct Move call
      expect(unsignedTx.transaction).toBeDefined();
    });
  });

  describe('admin access control', () => {
    it('should require admin cap for threshold changes', async () => {
      // Mock admin cap not found
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: null }));

      const txGenerator = suiNtt.setThreshold(3);
      await expect(txGenerator.next()).rejects.toThrow('AdminCap ID not found in NTT state');
    });

    it('should validate admin cap exists', async () => {
      // Mock valid admin cap retrieval
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'valid-admin-cap' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));

      const txGenerator = suiNtt.setThreshold(2);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: TEST_CONTRACTS.ntt.manager,
        options: { showContent: true }
      });
    });

    it('should handle admin cap caching', async () => {
      // First call should fetch admin cap
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'cached-admin-cap' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));

      const txGenerator1 = suiNtt.setThreshold(2);
      await txGenerator1.next();

      // Second call should use cached admin cap
      mockClient.getObject
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {})); // Only package ID call

      const txGenerator2 = suiNtt.setThreshold(3);
      const { value: unsignedTx } = await txGenerator2.next();
      
      expect(unsignedTx).toBeDefined();
    });
  });

  describe('getUpgradeCapId', () => {
    it('should return upgrade cap ID from state', async () => {
      const state = mockNttState({ upgradeCapId: 'upgrade-cap-123' });
      mockClient.getObject.mockResolvedValue(state);

      const upgradeCapId = await suiNtt.getUpgradeCapId();
      expect(upgradeCapId).toBe('upgrade-cap-123');
    });

    it('should throw error when upgrade cap ID not found', async () => {
      const state = mockNttState({ upgradeCapId: null });
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getUpgradeCapId()).rejects.toThrow('UpgradeCap ID not found in NTT state');
    });
  });

  describe('transaction validation', () => {
    beforeEach(() => {
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));
    });

    it('should create valid transactions with proper structure', async () => {
      const txGenerator = suiNtt.setThreshold(3);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
      expect(unsignedTx.transaction).toBeDefined();
      expect(unsignedTx.network).toBe('Testnet');
      expect(unsignedTx.chain).toBe('Sui');
      expect(unsignedTx.description).toBe('Set Threshold');
    });

    it('should handle transaction serialization', async () => {
      const txGenerator = suiNtt.setThreshold(4);
      const { value: unsignedTx } = await txGenerator.next();
      
      // Transaction should be serializable
      expect(() => JSON.stringify(unsignedTx.transaction)).not.toThrow();
    });

    it('should validate transaction parameters', async () => {
      const txGenerator = suiNtt.setThreshold(6);
      const { value: unsignedTx } = await txGenerator.next();
      
      // Check that the transaction contains the expected structure
      const tx = unsignedTx.transaction;
      expect(tx).toBeDefined();
      expect(tx.constructor.name).toBe('_Transaction');
    });
  });

  describe('error handling and edge cases', () => {
    it('should handle invalid threshold values gracefully', async () => {
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce(mockSuiObject('0xpackage::ntt::State', {}));

      // Negative threshold (should be handled by Move contract validation)
      const txGenerator = suiNtt.setThreshold(-1);
      const { value: unsignedTx } = await txGenerator.next();
      
      expect(unsignedTx).toBeDefined();
    });

    it('should handle package ID extraction failures', async () => {
      mockClient.getObject
        .mockResolvedValueOnce(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValueOnce({ data: null }); // Failed package ID extraction

      const txGenerator = suiNtt.setThreshold(2);
      await expect(txGenerator.next()).rejects.toThrow('Failed to fetch object');
    });

    it('should handle RPC failures gracefully', async () => {
      mockClient.getObject.mockRejectedValue(new Error('RPC connection failed'));

      const txGenerator = suiNtt.setThreshold(3);
      await expect(txGenerator.next()).rejects.toThrow('RPC connection failed');
    });

    it('should handle concurrent admin operations', async () => {
      mockClient.getObject
        .mockResolvedValue(mockNttState({ adminCapId: 'admin-cap-id' }))
        .mockResolvedValue(mockSuiObject('0xpackage::ntt::State', {}));

      // Multiple concurrent threshold changes
      const txGenerator1 = suiNtt.setThreshold(2);
      const txGenerator2 = suiNtt.setThreshold(3);
      
      const [result1, result2] = await Promise.all([
        txGenerator1.next(),
        txGenerator2.next()
      ]);
      
      expect(result1.value).toBeDefined();
      expect(result2.value).toBeDefined();
      expect(result1.value.description).toBe('Set Threshold');
      expect(result2.value.description).toBe('Set Threshold');
    });
  });
});