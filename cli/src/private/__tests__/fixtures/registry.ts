// Registry fixtures. RETIRED_CANBURN_REGISTRY holds the 2026-08 canburn values that the
// guardian-signed VAAs in vaas.ts reference. That deployment is retired; use it for tests only.
export const RETIRED_CANTON_NAMESPACE =
  "122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e";

export const TEST_NTT_PACKAGE_ID =
  "NTT_ID_TBD";
export const TEST_CORE_PACKAGE_ID =
  "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371";

export function retiredCanburnRegistryJson(): any {
  return {
    schemaVersion: 2,
    network: "Mainnet",
    chainIds: { Canton: 75, Ethereum: 2, Solana: 1 },
    canton: {
      disclosureServiceUrl: "https://canton-disclosure.example.test",
      participant: {
        jsonApiUrl: "https://participant.example.test",
        oauthTokenUrl:
          "https://idp.example.test/realms/test/protocol/openid-connect/token",
        oauthClientId: "test-client",
        userPartyId: `alice::${RETIRED_CANTON_NAMESPACE}`,
      },
    },
    deployments: {
      canburn: {
        chains: {
          Canton: {
            mode: "burning",
            decimals: 10,
            manager:
              "d03c78c99b240126e5e18bf07bae40847bb41b345b1850c71afb75e809f24d08",
            emitter:
              "b35f6b925a765a5ea9db67f640352c7fed7a8fda0ccd5f21ec53d984281755af",
            instrument: {
              admin: `guardianGovernance::${RETIRED_CANTON_NAMESPACE}`,
              id: "wormhole-ntt:d8740ea9950b48eb490f6aa9bd91e5ccb4066971482e476a6bb98cbff10a4e7d",
            },
            reserveCustodian: `nttCustody::${RETIRED_CANTON_NAMESPACE}`,
            vettedPackageIds: {
              ntt: TEST_NTT_PACKAGE_ID,
              core: TEST_CORE_PACKAGE_ID,
            },
          },
          Ethereum: {
            mode: "burning",
            decimals: 18,
            token: "0x88c4f2412c1f2CA5D459E3355C44F04be7bA8cA8",
            manager: "0xfd2322c8b9D79c605A4263F1baCeC9a0B4Ff6d42",
            transceiver: "0x06803b132816e8F6d277Ad7984305B2F590b038D",
            emitter:
              "00000000000000000000000006803b132816e8f6d277ad7984305b2f590b038d",
          },
          Solana: {
            mode: "burning",
            decimals: 9,
            idlVersion: "3.0.0",
            coreBridge: "worMq7dr1fGM24dUV2ciwMShz3p979L8LNcat9AhKsj",
            manager: "CBUrnuMWaMMDMWiJhmskaGmnRxH9Ye1DC6rvVNtqKKRt",
            mint: "HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1Gciw",
            transceiver: "6knb13neqpTUKTkVru4aFdhvSJf1Q8MAxJEEaXx2aQLU",
            emitter:
              "5581f43488b27e6c320d081523c04beefc1e58f7eee7576b1631a440adac453d",
          },
        },
      },
    },
  };
}
