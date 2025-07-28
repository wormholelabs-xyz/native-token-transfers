// Global test setup
// import 'jest-extended'; // Not available, using built-in Jest matchers

// Increase timeout for async operations
jest.setTimeout(30000);

// Mock console methods to reduce noise in tests
global.console = {
  ...console,
  log: jest.fn(),
  warn: jest.fn(),
  // Keep error for debugging
  error: console.error,
};