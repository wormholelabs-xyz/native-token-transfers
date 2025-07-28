import { SuiNtt } from '../src/ntt';
import { 
  mockSuiClient, 
  mockNttState, 
  mockAdminCap,
  mockUpgradeCap,
  TEST_ADDRESSES,
  TEST_CONTRACTS
} from './mocks';

describe('SuiNtt Core State Functions', () => {
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

  describe('constructor', () => {
    it('should create SuiNtt instance with correct properties', () => {
      expect(suiNtt.network).toBe('Testnet');
      expect(suiNtt.chain).toBe('Sui');
      expect(suiNtt.provider).toBe(mockClient);
      expect(suiNtt.contracts.ntt?.manager).toBe(TEST_CONTRACTS.ntt.manager);
      expect(suiNtt.contracts.ntt?.token).toBe(TEST_CONTRACTS.ntt.token);
    });

    it('should throw error when NTT contracts not found', () => {
      expect(() => {
        new SuiNtt('Testnet', 'Sui', mockClient, {
          coreBridge: TEST_CONTRACTS.coreBridge
        });
      }).toThrow('NTT contracts not found');
    });

    it('should throw error when Core Bridge contract not found', () => {
      expect(() => {
        new SuiNtt('Testnet', 'Sui', mockClient, {
          ntt: TEST_CONTRACTS.ntt
        });
      }).toThrow('Core Bridge contract not found');
    });
  });

  describe('getMode', () => {
    it('should return locking mode when state has Locking variant', async () => {
      const state = mockNttState({ mode: 'Locking' });
      mockClient.getObject.mockResolvedValue(state);

      const mode = await suiNtt.getMode();
      expect(mode).toBe('locking');
      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: TEST_CONTRACTS.ntt.manager,
        options: { showContent: true }
      });
    });

    it('should return burning mode when state has Burning variant', async () => {
      const state = mockNttState({ mode: 'Burning' });
      mockClient.getObject.mockResolvedValue(state);

      const mode = await suiNtt.getMode();
      expect(mode).toBe('burning');
    });

    it('should throw error when state fetch fails', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getMode()).rejects.toThrow('Failed to fetch NTT state object');
    });

    it('should throw error when mode is invalid', async () => {
      const state = mockNttState({ mode: 'InvalidMode' });
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getMode()).rejects.toThrow('Invalid mode in NTT state');
    });
  });

  describe('isPaused', () => {
    it('should return false as Sui NTT uses AdminCap access control', async () => {
      const result = await suiNtt.isPaused();
      expect(result).toBe(false);
    });
  });

  describe('getAdminCapId', () => {
    it('should return admin cap ID from state', async () => {
      const state = mockNttState({ adminCapId: 'test-admin-cap-id' });
      mockClient.getObject.mockResolvedValue(state);

      const adminCapId = await suiNtt.getAdminCapId();
      expect(adminCapId).toBe('test-admin-cap-id');
    });

    it('should cache admin cap ID after first fetch', async () => {
      const state = mockNttState({ adminCapId: 'cached-admin-cap-id' });
      mockClient.getObject.mockResolvedValue(state);

      // First call
      const adminCapId1 = await suiNtt.getAdminCapId();
      // Second call
      const adminCapId2 = await suiNtt.getAdminCapId();

      expect(adminCapId1).toBe('cached-admin-cap-id');
      expect(adminCapId2).toBe('cached-admin-cap-id');
      expect(mockClient.getObject).toHaveBeenCalledTimes(1);
    });

    it('should throw error when admin cap ID not found', async () => {
      const state = mockNttState({ adminCapId: null });
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getAdminCapId()).rejects.toThrow('AdminCap ID not found in NTT state');
    });
  });

  describe('getPackageId', () => {
    it('should extract package ID from state object type', async () => {
      const state = mockNttState();
      state.data.content.type = '0xabc123::ntt::State<0x2::sui::SUI>';
      mockClient.getObject.mockResolvedValue(state);

      const packageId = await suiNtt.getPackageId();
      expect(packageId).toBe('0xabc123');
    });

    it('should get latest package ID from upgrade cap when available', async () => {
      const state = mockNttState({ upgradeCapId: 'upgrade-cap-id' });
      state.data.content.type = '0xoldpackage::ntt::State<0x2::sui::SUI>';
      state.data.content.fields.upgrade_cap_id = 'upgrade-cap-id';
      
      const upgradeCap = mockUpgradeCap();
      upgradeCap.data.content.fields.cap.fields.package = '0xnewpackage';

      mockClient.getObject
        .mockResolvedValueOnce(state) // First call for state
        .mockResolvedValueOnce(upgradeCap); // Second call for upgrade cap

      const packageId = await suiNtt.getPackageId();
      expect(packageId).toBe('0xnewpackage');
    });

    it('should cache package ID after first fetch', async () => {
      const state = mockNttState();
      state.data.content.type = '0xcached::ntt::State<0x2::sui::SUI>';
      mockClient.getObject.mockResolvedValue(state);

      const packageId1 = await suiNtt.getPackageId();
      const packageId2 = await suiNtt.getPackageId();

      expect(packageId1).toBe('0xcached');
      expect(packageId2).toBe('0xcached');
      expect(mockClient.getObject).toHaveBeenCalledTimes(1);
    });

    it('should throw error when package ID cannot be extracted', async () => {
      const state = mockNttState();
      state.data.content.type = 'invalid::format';
      mockClient.getObject.mockResolvedValue(state);

      await expect(suiNtt.getPackageId()).rejects.toThrow('Could not extract package ID from state object type');
    });
  });

  describe('getOwner', () => {
    it('should return owner address from AdminCap', async () => {
      const state = mockNttState({ adminCapId: 'admin-cap-123' });
      const adminCap = mockAdminCap(TEST_ADDRESSES.ADMIN);
      
      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      const owner = await suiNtt.getOwner();
      expect(owner).toBe(TEST_ADDRESSES.ADMIN);
      
      expect(mockClient.getObject).toHaveBeenCalledWith({
        id: 'admin-cap-123',
        options: { showOwner: true }
      });
    });

    it('should handle string owner format', async () => {
      const state = mockNttState({ adminCapId: 'admin-cap-123' });
      const adminCap = {
        data: {
          owner: TEST_ADDRESSES.USER
        }
      };
      
      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      const owner = await suiNtt.getOwner();
      expect(owner).toBe(TEST_ADDRESSES.USER);
    });

    it('should throw error when AdminCap owner not found', async () => {
      const state = mockNttState({ adminCapId: 'admin-cap-123' });
      const adminCap = { data: { owner: null } };
      
      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      await expect(suiNtt.getOwner()).rejects.toThrow('Could not fetch AdminCap owner information');
    });

    it('should throw error on unexpected owner type', async () => {
      const state = mockNttState({ adminCapId: 'admin-cap-123' });
      const adminCap = { data: { owner: { UnknownType: 'value' } } };
      
      mockClient.getObject
        .mockResolvedValueOnce(state)
        .mockResolvedValueOnce(adminCap);

      await expect(suiNtt.getOwner()).rejects.toThrow('AdminCap has unexpected owner type');
    });
  });

  describe('getPauser', () => {
    it('should return null as not implemented', async () => {
      const result = await suiNtt.getPauser();
      expect(result).toBeNull();
    });
  });

  describe('getThreshold', () => {
    it('should return threshold value from state', async () => {
      const state = mockNttState({ threshold: '5' });
      mockClient.getObject.mockResolvedValue(state);

      const threshold = await suiNtt.getThreshold();
      expect(threshold).toBe(5);
    });

    it('should handle threshold as number', async () => {
      const state = mockNttState({ threshold: 3 });
      mockClient.getObject.mockResolvedValue(state);

      const threshold = await suiNtt.getThreshold();
      expect(threshold).toBe(3);
    });

    it('should throw error when state fetch fails', async () => {
      mockClient.getObject.mockResolvedValue({ data: null });

      await expect(suiNtt.getThreshold()).rejects.toThrow('Failed to fetch NTT state object');
    });
  });

  describe('getTokenDecimals', () => {
    it('should return 9 for SUI token', async () => {
      const decimals = await suiNtt.getTokenDecimals();
      expect(decimals).toBe(9);
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

      await expect(customSuiNtt.getTokenDecimals()).rejects.toThrow(
        'getTokenDecimals not yet implemented for token: 0xabc::custom::TOKEN'
      );
    });
  });

  describe('getCustodyAddress', () => {
    it('should return NTT manager address as custody address', async () => {
      const custodyAddress = await suiNtt.getCustodyAddress();
      expect(custodyAddress).toBe(TEST_CONTRACTS.ntt.manager);
    });
  });

  describe('getRateLimitDuration', () => {
    it('should return 24 hours in milliseconds', async () => {
      const duration = await suiNtt.getRateLimitDuration();
      expect(duration).toBe(BigInt(24 * 60 * 60 * 1000));
    });
  });

  describe('error handling', () => {
    it('should handle RPC errors gracefully', async () => {
      mockClient.getObject.mockRejectedValue(new Error('RPC connection failed'));

      await expect(suiNtt.getMode()).rejects.toThrow('RPC connection failed');
    });

    it('should handle malformed responses', async () => {
      mockClient.getObject.mockResolvedValue({
        data: {
          content: {
            dataType: 'package', // Wrong type
            fields: {}
          }
        }
      });

      await expect(suiNtt.getMode()).rejects.toThrow('Failed to fetch NTT state object');
    });
  });
});