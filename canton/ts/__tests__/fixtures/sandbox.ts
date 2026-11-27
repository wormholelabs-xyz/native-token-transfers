// Recorded on a local Canton 3.7.3 sandbox (dpm sandbox) with the NTT flow service
// (canton/disclosure/service) in front of it: the setup of __tests__/integration/daml
// (SdkSetup:sdkSetup), then the integration test (__tests__/integration/sandbox.test.ts) with
// CANTON_NTT_FIXTURE_OUT set. Party ids and contract ids are from that sandbox only.
// Regenerate with the integration test; do not edit by hand.
export const SANDBOX = {
  setup: {
    owned: {
      admin:
        "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      instrumentAdmin:
        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      managerAddress:
        "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
      tokenDecimals: 10,
      instrumentId: "sdk-owned-s5",
      reserveCustodian:
        "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      transceiverAddress:
        "547c1aa48d0d7e18df06baf8c641dde7bf8437af891da67112845fe919e31f49",
    },
    guardianGovernance:
      "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
    burning: {
      admin:
        "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      instrumentAdmin:
        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      managerAddress:
        "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
      tokenDecimals: 8,
      instrumentId:
        "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
      reserveCustodian:
        "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      transceiverAddress:
        "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
    },
    locking: {
      admin:
        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      instrumentAdmin:
        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      managerAddress:
        "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
      tokenDecimals: 10,
      instrumentId: "sdk-lock-s5",
      reserveCustodian:
        "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      transceiverAddress:
        "2597b4560bdfa6591974e049a43e316d69d95d6fa053d66f352e8e3473550353",
    },
    nttCustody:
      "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
    user: "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
    other:
      "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
  },
  managers: {
    managers: [
      {
        managerAddress:
          "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
        contractId:
          "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
        chainId: 75,
        managerId: "0",
        tokenConfig: "BurnMint",
        instrumentId: {
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
        },
        reserveCustodian:
          "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        paused: false,
        tokenDecimals: 8,
        admin:
          "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        peers: {
          "2": {
            managerAddress:
              "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            transceiverAddress:
              "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            decimals: 18,
          },
        },
      },
      {
        managerAddress:
          "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
        contractId:
          "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
        chainId: 75,
        managerId: "1",
        tokenConfig: "LockUnlock",
        instrumentId: {
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          id: "sdk-lock-s5",
        },
        reserveCustodian:
          "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        paused: false,
        tokenDecimals: 10,
        admin:
          "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        peers: {
          "2": {
            managerAddress:
              "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            transceiverAddress:
              "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            decimals: 18,
          },
        },
      },
      {
        managerAddress:
          "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
        contractId:
          "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
        chainId: 75,
        managerId: "2",
        tokenConfig: "LockUnlock",
        instrumentId: {
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          id: "sdk-owned-s5",
        },
        reserveCustodian:
          "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        paused: false,
        tokenDecimals: 10,
        admin:
          "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        peers: {
          "2": {
            managerAddress:
              "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            transceiverAddress:
              "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
            decimals: 18,
          },
        },
      },
    ],
  },
  burnTransfer: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            choice: "Transfer",
            choiceArgument: {
              user: "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transceiverEmitterCid:
                "001308d051bb912f5b36ba5506dd39944244e78387dc86660203e75364543020fdca121220bcf6667987c0fb426a80e0a659178072a72eaa28593bf6bd09472674c1dbcf98",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              ledgerCid: null,
              recipientChain: "2",
              recipientAddress: {
                tag: "UniversalAddress",
                value:
                  "00000000000000000000000000000000000000000000000000000000000000aa",
              },
              rawAmount: "100000000",
              nonce: "721041016",
              consistencyLevel: "0",
              inputHoldingCids: [
                "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
              ],
              feeAllocation: null,
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
              expectedManagerAddress: {
                tag: "NttManagerAddress",
                value:
                  "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
              },
            },
          },
        },
      ],
      commandId:
        "ntt-private-send-75d115eec7a9f2da0994abcca1ab3eb397d4e9c597466441bddad10762f09ac3",
      actAs: [
        "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          createdEventBlob:
            "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "001308d051bb912f5b36ba5506dd39944244e78387dc86660203e75364543020fdca121220bcf6667987c0fb426a80e0a659178072a72eaa28593bf6bd09472674c1dbcf98",
          createdEventBlob:
            "CgMyLjESngUKRQATCNBRu5EvWza6VQbdOZRCROeDh9yGZgID51NkVDAg/coSEiC89mZ5h8D7QmqA4KZZF4Bypy6qKFk79r0JRyZ0wdvPmBINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjkWZjl1DWEGAEIqCiYKJAgBEiDU6/E3tzf1s9UL8HPEcSpH981dPLDGx70APqSEU5Lg0xAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    flowUrl:
      "/v1/flows/transfer?manager=00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
    flow: {
      flow: "transfer",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          createdEventBlob:
            "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
            admin:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "0",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
            },
            transceiverEmitterId: "0",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
            },
            tokenConfig: "BurnMint",
            tokenDecimals: "8",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "BurnMintFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "transceiver Emitter",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "001308d051bb912f5b36ba5506dd39944244e78387dc86660203e75364543020fdca121220bcf6667987c0fb426a80e0a659178072a72eaa28593bf6bd09472674c1dbcf98",
          createdEventBlob:
            "CgMyLjESngUKRQATCNBRu5EvWza6VQbdOZRCROeDh9yGZgID51NkVDAg/coSEiC89mZ5h8D7QmqA4KZZF4Bypy6qKFk79r0JRyZ0wdvPmBINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjkWZjl1DWEGAEIqCiYKJAgBEiDU6/E3tzf1s9UL8HPEcSpH981dPLDGx70APqSEU5Lg0xAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            owner:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            emitterId: "0",
            emitterAddress: {
              tag: "EmitterAddress",
              value:
                "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
            },
            sequence: "2",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "committed canonical CoinFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
      ],
      missing: [],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
    holdings: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 480,
              nodeId: 1,
              contractId:
                "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "100.0000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "sdk-lock-s5",
                    },
                    amount: "100.0000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:53.168624Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIxAUSBHOyZ3I=",
      },
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 483,
              nodeId: 9,
              contractId:
                "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "2.5000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                    },
                    amount: "2.5000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:59.718243Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIzAUSBHOyZ3I=",
      },
    ],
    transaction: {
      updateId:
        "12207612ea18b77aca6f022da7c61c29272305bf2de27a24949889c53b3b0ea6e736",
      commandId:
        "ntt-private-send-75d115eec7a9f2da0994abcca1ab3eb397d4e9c597466441bddad10762f09ac3",
      workflowId: "",
      effectiveAt: "2026-11-24T22:45:05.793479Z",
      events: [
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 0,
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            interfaceId: null,
            choice: "Transfer",
            choiceArgument: {
              user: "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transceiverEmitterCid:
                "001308d051bb912f5b36ba5506dd39944244e78387dc86660203e75364543020fdca121220bcf6667987c0fb426a80e0a659178072a72eaa28593bf6bd09472674c1dbcf98",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              ledgerCid: null,
              recipientChain: "2",
              recipientAddress: {
                tag: "UniversalAddress",
                value:
                  "00000000000000000000000000000000000000000000000000000000000000aa",
              },
              rawAmount: "100000000",
              nonce: "721041016",
              consistencyLevel: "0",
              inputHoldingCids: [
                "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
              ],
              feeAllocation: null,
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
              expectedManagerAddress: {
                tag: "NttManagerAddress",
                value:
                  "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
              },
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 10,
            exerciseResult: {
              registrar:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "0",
              sequence: "2",
              nonce: "721041016",
              consistencyLevel: "0",
              payload: {
                tag: "MessagePayload",
                value:
                  "9945ff1000469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000005f5e1002b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d809200000000000000000000000000000000000000000000000000000000000000aa00020000",
              },
            },
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 3,
            contractId:
              "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
            interfaceId:
              "9cc2cbc838ef38dc2c7f34014c9c452bcf71b8e2a4f939235fc0b5d0924b185e:Splice.Api.Token.BurnMintV1:BurnMintFactory",
            choice: "BurnMintFactory_BurnMint",
            choiceArgument: {
              expectedAdmin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              inputHoldingCids: [
                "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
              ],
              outputs: [
                {
                  owner:
                    "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  amount: "1.5000000000",
                  context: {
                    values: {},
                  },
                },
              ],
              extraActors: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 6,
            exerciseResult: {
              outputCids: [
                "008f012e25ef7ce26d0a9f1d3675545c11c1015d35f2db75e95d244cff3298c04dca121220f807ad00951cfce4cc24d20c3328eaa230309c53cfd4aa7a3b3e68d5707507eb",
              ],
            },
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 5,
            contractId:
              "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            interfaceId:
              "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
            choice: "Archive",
            choiceArgument: {},
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 5,
            exerciseResult: {},
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: true,
          },
        },
        {
          CreatedEvent: {
            offset: 487,
            nodeId: 6,
            contractId:
              "008f012e25ef7ce26d0a9f1d3675545c11c1015d35f2db75e95d244cff3298c04dca121220f807ad00951cfce4cc24d20c3328eaa230309c53cfd4aa7a3b3e68d5707507eb",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              owner:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "1.5000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:05.793479Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: true,
          },
        },
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 7,
            contractId:
              "001308d051bb912f5b36ba5506dd39944244e78387dc86660203e75364543020fdca121220bcf6667987c0fb426a80e0a659178072a72eaa28593bf6bd09472674c1dbcf98",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            interfaceId: null,
            choice: "PublishMessage",
            choiceArgument: {
              nonce: "721041016",
              payload: {
                tag: "MessagePayload",
                value:
                  "9945ff1000469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000005f5e1002b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d809200000000000000000000000000000000000000000000000000000000000000aa00020000",
              },
              consistencyLevel: "0",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              payer:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 10,
            exerciseResult: {
              _1: "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
              _2: {
                registrar:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                owner:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                emitterId: "0",
                sequence: "2",
                nonce: "721041016",
                consistencyLevel: "0",
                payload: {
                  tag: "MessagePayload",
                  value:
                    "9945ff1000469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000005f5e1002b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d809200000000000000000000000000000000000000000000000000000000000000aa00020000",
                },
              },
            },
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 8,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "GetGuardianGovernance",
            choiceArgument: {
              reader:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 8,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 487,
            nodeId: 9,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "ChargeMessageFee",
            choiceArgument: {
              payer:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 9,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 487,
            nodeId: 10,
            contractId:
              "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "0",
              emitterAddress: {
                tag: "EmitterAddress",
                value:
                  "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
              },
              sequence: "3",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:05.793479Z",
            packageName: "wormhole-core",
            representativePackageId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371",
            acsDelta: false,
          },
        },
      ],
      offset: 487,
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      traceContext: {
        traceparent: "00-ff254f56e537c60c43cfe4da893740a2-b7d9316319062c76-03",
        tracestate: null,
      },
      recordTime: "2026-11-24T22:45:05.824474Z",
      externalTransactionHash: null,
      paidTrafficCost: 0,
      transactionHash: null,
    },
  },
  lockTransfer: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
            choice: "Transfer",
            choiceArgument: {
              user: "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transceiverEmitterCid:
                "0042d20a2e284bd8fed057b0006138c7c68e147df99805cbbfd9fcabdf37489085ca1212208a1767048a6138d246276dcf0c60a3af738b7297b178455c239cee0439e359cd",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              ledgerCid:
                "005182f4750d88a752d4f08e9f104db08b5dc0dc2d70bd40a2b4374264b63f1ba6ca121220b197cad4040fbd50ac35fb75808949f8fdc16aea6ac734a7e755d995ba11c552",
              recipientChain: "2",
              recipientAddress: {
                tag: "UniversalAddress",
                value:
                  "00000000000000000000000000000000000000000000000000000000000000aa",
              },
              rawAmount: "15000000000",
              nonce: "7",
              consistencyLevel: "0",
              inputHoldingCids: [
                "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
              ],
              feeAllocation: null,
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
              expectedManagerAddress: {
                tag: "NttManagerAddress",
                value:
                  "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
              },
            },
          },
        },
      ],
      commandId:
        "ntt-private-send-6d97ec8620cb878b7742e57afc2f1ab57736500bc6e46344e3edd677c93c3409",
      actAs: [
        "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          createdEventBlob:
            "CgMyLjES2QwKRQBm5hRP6EnBqzGYWKlPAhlLm0ZeooOn9paWx20c0segFcoSEiASOUxFSVoLaKO6lXYlrJHmKph7kLkMJQ4W+MuZUa94yRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIiowlqoAkKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAIKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKBAoCGAIKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNQoQCg56DAoKTG9ja1VubG9jawoECgIYFApaClhyVgoQVW5pdmVyc2FsQWRkcmVzcxJCQkBmODNjZDE0NzY2MjU5NjkwMjJkNDk4ODA3MWJjMWM4Y2VlZTQwYTQwMGY3ZmI4NDMwNmE2MDQ0NmMzZjc0NjQ2CtABCs0BYsoBCscBCgIYBBLAAWq9AQpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYjFmZjVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYzJhYTVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgoECgIYJAphCl9yXQoSVHJhbnNmZXJGYWN0b3J5Q2lkEkdKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuQoECgIQAAoECgIYAAoECgIYAipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgRZ1cBcHlFZ2jh9kHw0+yV/q9ayqiBuSc5vPcTArfjF0QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "0042d20a2e284bd8fed057b0006138c7c68e147df99805cbbfd9fcabdf37489085ca1212208a1767048a6138d246276dcf0c60a3af738b7297b178455c239cee0439e359cd",
          createdEventBlob:
            "CgMyLjESngUKRQBC0gouKEvY/tBXsABhOMfGjhR9+ZgFy7/Z/KvfN0iQhcoSEiCKF2cEimE40kYnbc8MYKOvc4tyl7F4RVwjnO4EOeNZzRINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMjU5N2I0NTYwYmRmYTY1OTE5NzRlMDQ5YTQzZTMxNmQ2OWQ5NWQ2ZmEwNTNkNjZmMzUyZThlMzQ3MzU1MDM1MwoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjm62kZ1DWEGAEIqCiYKJAgBEiDKUZnmywliOi8EWoQp0BKJbc/AJKAXifmV0sbEq7nnLRAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "005182f4750d88a752d4f08e9f104db08b5dc0dc2d70bd40a2b4374264b63f1ba6ca121220b197cad4040fbd50ac35fb75808949f8fdc16aea6ac734a7e755d995ba11c552",
          createdEventBlob:
            "CgMyLjESogUKRQBRgvR1DYinUtTwjp8QTbCLXcDcLXC9QKK0N0Jktj8bpsoSEiCxl8rUBA+9UKw1+3WAiUn4/cFq6mrHNKfnVdmVuhHFUhIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIswCaskCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKEAoOMgwwLjAwMDAwMDAwMDAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgn3pAJdhQTUAeFSRjne5S2BtWpxBFm0iGMYBO27Jp9tMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
          contractId:
            "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
          createdEventBlob:
            "CgMyLjESvQUKRQB53wNUb2wHRRqqx0oG1FBjbr26sFtgwARZO7YxsTdd4soSEiDj9MPOefBQRigqqrB8jCH0mgO3jk/LWfhJJJ8X9bXlDBINdG9rZW4tY2lwMDA1Nhp1CkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIMQ29pblRyYW5zZmVyGhNUcmFuc2ZlclByZWFwcHJvdmFsIu4BausBCmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNSpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYyZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5S1RedQ1hBgBCKgomCiQIARIgEcMnthGo5ZSyyllPgeWzMF3umxNcPm4UHxR4BiyrhRwQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    flowUrl:
      "/v1/flows/transfer?manager=26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
    flow: {
      flow: "transfer",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          createdEventBlob:
            "CgMyLjES2QwKRQBm5hRP6EnBqzGYWKlPAhlLm0ZeooOn9paWx20c0segFcoSEiASOUxFSVoLaKO6lXYlrJHmKph7kLkMJQ4W+MuZUa94yRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIiowlqoAkKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAIKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKBAoCGAIKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNQoQCg56DAoKTG9ja1VubG9jawoECgIYFApaClhyVgoQVW5pdmVyc2FsQWRkcmVzcxJCQkBmODNjZDE0NzY2MjU5NjkwMjJkNDk4ODA3MWJjMWM4Y2VlZTQwYTQwMGY3ZmI4NDMwNmE2MDQ0NmMzZjc0NjQ2CtABCs0BYsoBCscBCgIYBBLAAWq9AQpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYjFmZjVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYzJhYTVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgoECgIYJAphCl9yXQoSVHJhbnNmZXJGYWN0b3J5Q2lkEkdKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuQoECgIQAAoECgIYAAoECgIYAipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgRZ1cBcHlFZ2jh9kHw0+yV/q9ayqiBuSc5vPcTArfjF0QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "1",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
            },
            transceiverEmitterId: "1",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-lock-s5",
            },
            tokenConfig: "LockUnlock",
            tokenDecimals: "10",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f74646",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "TransferFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "transceiver Emitter",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "0042d20a2e284bd8fed057b0006138c7c68e147df99805cbbfd9fcabdf37489085ca1212208a1767048a6138d246276dcf0c60a3af738b7297b178455c239cee0439e359cd",
          createdEventBlob:
            "CgMyLjESngUKRQBC0gouKEvY/tBXsABhOMfGjhR9+ZgFy7/Z/KvfN0iQhcoSEiCKF2cEimE40kYnbc8MYKOvc4tyl7F4RVwjnO4EOeNZzRINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMjU5N2I0NTYwYmRmYTY1OTE5NzRlMDQ5YTQzZTMxNmQ2OWQ5NWQ2ZmEwNTNkNjZmMzUyZThlMzQ3MzU1MDM1MwoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjm62kZ1DWEGAEIqCiYKJAgBEiDKUZnmywliOi8EWoQp0BKJbc/AJKAXifmV0sbEq7nnLRAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            owner:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            emitterId: "1",
            emitterAddress: {
              tag: "EmitterAddress",
              value:
                "2597b4560bdfa6591974e049a43e316d69d95d6fa053d66f352e8e3473550353",
            },
            sequence: "2",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "LockedLedger",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "005182f4750d88a752d4f08e9f104db08b5dc0dc2d70bd40a2b4374264b63f1ba6ca121220b197cad4040fbd50ac35fb75808949f8fdc16aea6ac734a7e755d995ba11c552",
          createdEventBlob:
            "CgMyLjESogUKRQBRgvR1DYinUtTwjp8QTbCLXcDcLXC9QKK0N0Jktj8bpsoSEiCxl8rUBA+9UKw1+3WAiUn4/cFq6mrHNKfnVdmVuhHFUhIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIswCaskCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKEAoOMgwwLjAwMDAwMDAwMDAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgn3pAJdhQTUAeFSRjne5S2BtWpxBFm0iGMYBO27Jp9tMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
            },
            balance: "0.0000000000",
          },
        },
        {
          role: "committed TransferFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
        {
          role: "custodian TransferPreapproval",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
          contractId:
            "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
          createdEventBlob:
            "CgMyLjESvQUKRQB53wNUb2wHRRqqx0oG1FBjbr26sFtgwARZO7YxsTdd4soSEiDj9MPOefBQRigqqrB8jCH0mgO3jk/LWfhJJJ8X9bXlDBINdG9rZW4tY2lwMDA1Nhp1CkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIMQ29pblRyYW5zZmVyGhNUcmFuc2ZlclByZWFwcHJvdmFsIu4BausBCmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNSpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYyZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5S1RedQ1hBgBCKgomCiQIARIgEcMnthGo5ZSyyllPgeWzMF3umxNcPm4UHxR4BiyrhRwQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            owner:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-lock-s5",
            },
          },
        },
      ],
      missing: [],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
    holdings: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 480,
              nodeId: 1,
              contractId:
                "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "100.0000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "sdk-lock-s5",
                    },
                    amount: "100.0000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:53.168624Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIxAUSBNXpHJs=",
      },
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 487,
              nodeId: 6,
              contractId:
                "008f012e25ef7ce26d0a9f1d3675545c11c1015d35f2db75e95d244cff3298c04dca121220f807ad00951cfce4cc24d20c3328eaa230309c53cfd4aa7a3b3e68d5707507eb",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "1.5000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                    },
                    amount: "1.5000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:45:05.793479Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMI0AUSBNXpHJs=",
      },
    ],
    transaction: {
      updateId:
        "1220dc49242cd95c39eada7e02579f7008ea091f561b430ea564b48665e59be43c94",
      commandId:
        "ntt-private-send-6d97ec8620cb878b7742e57afc2f1ab57736500bc6e46344e3edd677c93c3409",
      workflowId: "",
      effectiveAt: "2026-11-24T22:45:06.585486Z",
      events: [
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 0,
            contractId:
              "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            interfaceId: null,
            choice: "Transfer",
            choiceArgument: {
              user: "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transceiverEmitterCid:
                "0042d20a2e284bd8fed057b0006138c7c68e147df99805cbbfd9fcabdf37489085ca1212208a1767048a6138d246276dcf0c60a3af738b7297b178455c239cee0439e359cd",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              ledgerCid:
                "005182f4750d88a752d4f08e9f104db08b5dc0dc2d70bd40a2b4374264b63f1ba6ca121220b197cad4040fbd50ac35fb75808949f8fdc16aea6ac734a7e755d995ba11c552",
              recipientChain: "2",
              recipientAddress: {
                tag: "UniversalAddress",
                value:
                  "00000000000000000000000000000000000000000000000000000000000000aa",
              },
              rawAmount: "15000000000",
              nonce: "7",
              consistencyLevel: "0",
              inputHoldingCids: [
                "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
              ],
              feeAllocation: null,
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
              expectedManagerAddress: {
                tag: "NttManagerAddress",
                value:
                  "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
              },
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 16,
            exerciseResult: {
              registrar:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "1",
              sequence: "2",
              nonce: "7",
              consistencyLevel: "0",
              payload: {
                tag: "MessagePayload",
                value:
                  "9945ff1026f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000008f0d180f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f7464600000000000000000000000000000000000000000000000000000000000000aa00020000",
              },
            },
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 3,
            contractId:
              "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
            interfaceId:
              "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferFactory",
            choice: "TransferFactory_Transfer",
            choiceArgument: {
              expectedAdmin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transfer: {
                sender:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                receiver:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "1.5000000000",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                requestedAt: "2026-11-24T22:45:06.585486Z",
                executeBefore: "2026-11-25T22:45:06.585486Z",
                inputHoldingCids: [
                  "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
                ],
                meta: {
                  values: {},
                },
              },
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 8,
            exerciseResult: {
              output: {
                tag: "TransferInstructionResult_Completed",
                value: {
                  receiverHoldingCids: [
                    "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
                  ],
                },
              },
              senderChangeCids: [
                "006cdf2f7e190a2f37b3a3babd6b8948d5d981e46be6f2d65cf3e44bb600590ddcca1212208e5958186b4eef7fed2bc6a25716c63c063e98af3dd361003dfe94f819d9818a",
              ],
              meta: {
                values: {},
              },
            },
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 5,
            contractId:
              "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            interfaceId:
              "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
            choice: "Archive",
            choiceArgument: {},
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 5,
            exerciseResult: {},
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: true,
          },
        },
        {
          CreatedEvent: {
            offset: 490,
            nodeId: 6,
            contractId:
              "006cdf2f7e190a2f37b3a3babd6b8948d5d981e46be6f2d65cf3e44bb600590ddcca1212208e5958186b4eef7fed2bc6a25716c63c063e98af3dd361003dfe94f819d9818a",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
              owner:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "98.5000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:06.585486Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: true,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 7,
            contractId:
              "0079df03546f6c07451aaac74a06d450636ebdbab05b60c004593bb631b1375de2ca121220e3f4c3ce79f05046282aaab07c8c21f49a03b78e4fcb59f849249f17f5b5e50c",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
            interfaceId: null,
            choice: "TransferPreapproval_Receive",
            choiceArgument: {
              receiver:
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "1.5000000000",
              expectedInstrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 8,
            exerciseResult:
              "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 490,
            nodeId: 8,
            contractId:
              "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
              owner:
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "1.5000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:06.585486Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 11,
            contractId:
              "005182f4750d88a752d4f08e9f104db08b5dc0dc2d70bd40a2b4374264b63f1ba6ca121220b197cad4040fbd50ac35fb75808949f8fdc16aea6ac734a7e755d995ba11c552",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
            interfaceId: null,
            choice: "Credit",
            choiceArgument: {
              amount: "1.5000000000",
              newCustodyHint:
                "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 12,
            exerciseResult:
              "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 490,
            nodeId: 12,
            contractId:
              "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              managerAddress: {
                tag: "NttManagerAddress",
                value:
                  "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
              },
              balance: "1.5000000000",
              custodyHoldingCid:
                "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:06.585486Z",
            packageName: "ntt",
            representativePackageId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 13,
            contractId:
              "0042d20a2e284bd8fed057b0006138c7c68e147df99805cbbfd9fcabdf37489085ca1212208a1767048a6138d246276dcf0c60a3af738b7297b178455c239cee0439e359cd",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            interfaceId: null,
            choice: "PublishMessage",
            choiceArgument: {
              nonce: "7",
              payload: {
                tag: "MessagePayload",
                value:
                  "9945ff1026f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000008f0d180f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f7464600000000000000000000000000000000000000000000000000000000000000aa00020000",
              },
              consistencyLevel: "0",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              payer:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 16,
            exerciseResult: {
              _1: "000a9f0dfe73d5926229fbb728d3eb76d7491ad5cd8f3226a184716f42312b2149ca121220f67a2e8261970effbc6812779a5e8be1b22d7ce947a35c8db2842e61e6eb9d4d",
              _2: {
                registrar:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                owner:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                emitterId: "1",
                sequence: "2",
                nonce: "7",
                consistencyLevel: "0",
                payload: {
                  tag: "MessagePayload",
                  value:
                    "9945ff1026f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00910000000000000000000000000000000000000000000000000000000000000002eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004f994e5454080000000008f0d180f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f7464600000000000000000000000000000000000000000000000000000000000000aa00020000",
                },
              },
            },
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 14,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "GetGuardianGovernance",
            choiceArgument: {
              reader:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 14,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 490,
            nodeId: 15,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "ChargeMessageFee",
            choiceArgument: {
              payer:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 15,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 490,
            nodeId: 16,
            contractId:
              "000a9f0dfe73d5926229fbb728d3eb76d7491ad5cd8f3226a184716f42312b2149ca121220f67a2e8261970effbc6812779a5e8be1b22d7ce947a35c8db2842e61e6eb9d4d",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "1",
              emitterAddress: {
                tag: "EmitterAddress",
                value:
                  "2597b4560bdfa6591974e049a43e316d69d95d6fa053d66f352e8e3473550353",
              },
              sequence: "3",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:06.585486Z",
            packageName: "wormhole-core",
            representativePackageId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371",
            acsDelta: false,
          },
        },
      ],
      offset: 490,
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      traceContext: {
        traceparent: "00-3b80f75629aa85af7c4201d3e270186b-625f7b7b829b311a-03",
        tracestate: null,
      },
      recordTime: "2026-11-24T22:45:06.620087Z",
      externalTransactionHash: null,
      paidTrafficCost: 0,
      transactionHash: null,
    },
  },
  mint: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            choice: "Mint",
            choiceArgument: {
              executor:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              replayNodeCid:
                "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
              vaaBytes: {
                tag: "EncodedVaa",
                value:
                  "010000000001008ded904ccae57e94833cf1bb59636b1f1872a262819fcc264182033b307a9b0063bb1bab77797c29937ca89c52d1042ed296cbae33ae071bae1d30050aece1a6006b06136b000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d75d5d5cec89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              recipient:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              depositPreapprovalCid:
                "00c1657d7245b7ef3456f8f10b27dc04c736dd70687b304d8a34507774c6b6ecdeca12122090abd4dd3c5342698e008538d0b69e591b285cc0aa8eec8fbdb89eeb6800b4ed",
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
          },
        },
      ],
      commandId:
        "ntt-private-redeem-77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
      actAs: [
        "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          createdEventBlob:
            "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
          contractId:
            "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
          createdEventBlob:
            "CgMyLjEStwUKRQD5MNEfEYmZuSE/vGmY7hrD36ua4RjIvIC4NhdivPFv0coSEiDRdYquwRZELiF8/KUnTTN/AgqIdd96TIkngeqfxf4tlRINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlItgCatUCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKCgoIagYKBAoCYgAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5Li4zdQ1hBgBCKgomCiQIARIgBMaasnhmx+HD0BJBz7JcZ0g5/VQ3Aq9bQjmORjnzbDkQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Deposit:DepositPreapproval",
          contractId:
            "00c1657d7245b7ef3456f8f10b27dc04c736dd70687b304d8a34507774c6b6ecdeca12122090abd4dd3c5342698e008538d0b69e591b285cc0aa8eec8fbdb89eeb6800b4ed",
          createdEventBlob:
            "CgMyLjES4wUKRQDBZX1yRbfvNFb48Qsn3ATHNt1waHswTYo0UHd0xrbs3soSEiCQq9TdPFNCaY4AhTjQtp5ZGyhcwKqO7I+9uJ7raAC07RIDbnR0Gm4KQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB0RlcG9zaXQaEkRlcG9zaXRQcmVhcHByb3ZhbCKrAmqoAgpdCls6WVNka1VzZXItczUtMDVlOGIxMDc6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CsYBCsMBasABCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpRCk9CTXdvcm1ob2xlLW50dDo3YjZlMDU4MzQ5NjQ3NTQ5MDE5MGZlZjkyN2Q0MmIxNzM4ODBkMTI4MTNkZThkMDcyMThjNDA5MWUxNTY1NWE4KllTZGtVc2VyLXM1LTA1ZThiMTA3OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjJnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnPTWl1DWEGAEIqCiYKJAgBEiBIgG5dWPlOO2Ox5c9qaWWWZHMP+yr1Ahb1kFL3Lp+9hhAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    flowUrl:
      "/v1/flows/mint?manager=00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382&digest=77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc&recipient=SdkUser-s5-05e8b107%3A%3A122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
    flow: {
      flow: "mint",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          createdEventBlob:
            "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
            admin:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "0",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
            },
            transceiverEmitterId: "0",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
            },
            tokenConfig: "BurnMint",
            tokenDecimals: "8",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "BurnMintFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "covering ReplayNode",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
          contractId:
            "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
          createdEventBlob:
            "CgMyLjEStwUKRQD5MNEfEYmZuSE/vGmY7hrD36ua4RjIvIC4NhdivPFv0coSEiDRdYquwRZELiF8/KUnTTN/AgqIdd96TIkngeqfxf4tlRINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlItgCatUCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKCgoIagYKBAoCYgAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5Li4zdQ1hBgBCKgomCiQIARIgBMaasnhmx+HD0BJBz7JcZ0g5/VQ3Aq9bQjmORjnzbDkQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            consumer:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            namespace:
              "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
            prefix: {
              tag: "DigestNibblePrefix",
              value: "",
            },
            splitThreshold: "128",
            consumed: {
              map: [],
            },
          },
        },
        {
          role: "DepositPreapproval",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Deposit:DepositPreapproval",
          contractId:
            "00c1657d7245b7ef3456f8f10b27dc04c736dd70687b304d8a34507774c6b6ecdeca12122090abd4dd3c5342698e008538d0b69e591b285cc0aa8eec8fbdb89eeb6800b4ed",
          createdEventBlob:
            "CgMyLjES4wUKRQDBZX1yRbfvNFb48Qsn3ATHNt1waHswTYo0UHd0xrbs3soSEiCQq9TdPFNCaY4AhTjQtp5ZGyhcwKqO7I+9uJ7raAC07RIDbnR0Gm4KQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB0RlcG9zaXQaEkRlcG9zaXRQcmVhcHByb3ZhbCKrAmqoAgpdCls6WVNka1VzZXItczUtMDVlOGIxMDc6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CsYBCsMBasABCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpRCk9CTXdvcm1ob2xlLW50dDo3YjZlMDU4MzQ5NjQ3NTQ5MDE5MGZlZjkyN2Q0MmIxNzM4ODBkMTI4MTNkZThkMDcyMThjNDA5MWUxNTY1NWE4KllTZGtVc2VyLXM1LTA1ZThiMTA3OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjJnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnPTWl1DWEGAEIqCiYKJAgBEiBIgG5dWPlOO2Ox5c9qaWWWZHMP+yr1Ahb1kFL3Lp+9hhAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            owner:
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
            },
          },
        },
        {
          role: "committed canonical CoinFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
      ],
      missing: [],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
    holdings: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 480,
              nodeId: 1,
              contractId:
                "00f3c33532496a3cbb1fcabf6c14893753d35cc3d01c0f44782a966021c90b34caca1212201c570138249f990a3d8dba50191493e1df0b7cb17c7edf94cbc7884e1450ce1a",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "100.0000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "sdk-lock-s5",
                    },
                    amount: "100.0000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:53.168624Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIxAUSBEI8jqc=",
      },
    ],
    transaction: {
      updateId:
        "1220433f9406e3b7352e2743281fb65b45ae69332d79393551751372fca3c91815e4",
      commandId:
        "ntt-private-redeem-77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
      workflowId: "",
      effectiveAt: "2026-11-24T22:44:59.718243Z",
      events: [
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 0,
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            interfaceId: null,
            choice: "Mint",
            choiceArgument: {
              executor:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              replayNodeCid:
                "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
              vaaBytes: {
                tag: "EncodedVaa",
                value:
                  "010000000001008ded904ccae57e94833cf1bb59636b1f1872a262819fcc264182033b307a9b0063bb1bab77797c29937ca89c52d1042ed296cbae33ae071bae1d30050aece1a6006b06136b000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d75d5d5cec89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              recipient:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              depositPreapprovalCid:
                "00c1657d7245b7ef3456f8f10b27dc04c736dd70687b304d8a34507774c6b6ecdeca12122090abd4dd3c5342698e008538d0b69e591b285cc0aa8eec8fbdb89eeb6800b4ed",
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 9,
            exerciseResult: {
              decimals: "8",
              amount: "250000000",
              sourceToken: {
                tag: "UniversalAddress",
                value:
                  "00000000000000000000000000000000000000000000000000000000000000dd",
              },
              recipientAddress: {
                tag: "UniversalAddress",
                value:
                  "eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02",
              },
              recipientChain: "75",
              additionalPayload: {
                tag: "HexBytes",
                value: "",
              },
            },
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 1,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "GetGuardianGovernance",
            choiceArgument: {
              reader:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 1,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 2,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "VerifyAndConsumeVAA",
            choiceArgument: {
              encodedVAA: {
                tag: "EncodedVaa",
                value:
                  "010000000001008ded904ccae57e94833cf1bb59636b1f1872a262819fcc264182033b307a9b0063bb1bab77797c29937ca89c52d1042ed296cbae33ae071bae1d30050aece1a6006b06136b000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d75d5d5cec89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              replayNodeCid:
                "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 5,
            exerciseResult: {
              _1: {
                version: "1",
                guardianSetIndex: "0",
                signatures: [
                  {
                    guardianIndex: "0",
                    r: {
                      tag: "EcdsaSignatureR",
                      value:
                        "8ded904ccae57e94833cf1bb59636b1f1872a262819fcc264182033b307a9b00",
                    },
                    s: {
                      tag: "EcdsaSignatureS",
                      value:
                        "63bb1bab77797c29937ca89c52d1042ed296cbae33ae071bae1d30050aece1a6",
                    },
                    v: "0",
                  },
                ],
                timestamp: "1795560299",
                nonce: "0",
                emitterChain: "2",
                emitterAddress: {
                  tag: "EmitterAddress",
                  value:
                    "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                },
                sequence: "1795560299681230",
                consistencyLevel: "200",
                payload: {
                  tag: "MessagePayload",
                  value:
                    "9945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
                },
                body: {
                  tag: "HexBytes",
                  value:
                    "6b06136b000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d75d5d5cec89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
                },
                hash: {
                  tag: "VaaDigest",
                  value:
                    "77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
                },
              },
              _2: "008243237430a5a3bd0335546b3034c8c68a463c4e1213cadf4d192c20cfc3b647ca1212207c1a05f851a14b661e33df6a0e50992b803e95fd4fe4cda0010b5318c5b4ef63",
            },
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 4,
            contractId:
              "00f930d11f118999b9213fbc6998ee1ac3dfab9ae118c8bc80b8361762bcf16fd1ca121220d1758aaec116442e217cfca5274d337f020a8875df7a4c892781ea9fc5fe2d95",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            interfaceId: null,
            choice: "ConsumeDigest",
            choiceArgument: {
              digest: {
                tag: "VaaDigest",
                value:
                  "77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 5,
            exerciseResult:
              "008243237430a5a3bd0335546b3034c8c68a463c4e1213cadf4d192c20cfc3b647ca1212207c1a05f851a14b661e33df6a0e50992b803e95fd4fe4cda0010b5318c5b4ef63",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 483,
            nodeId: 5,
            contractId:
              "008243237430a5a3bd0335546b3034c8c68a463c4e1213cadf4d192c20cfc3b647ca1212207c1a05f851a14b661e33df6a0e50992b803e95fd4fe4cda0010b5318c5b4ef63",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              prefix: {
                tag: "DigestNibblePrefix",
                value: "",
              },
              splitThreshold: "128",
              consumed: {
                map: [
                  [
                    {
                      tag: "DigestNibbleSuffix",
                      value:
                        "77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
                    },
                    {},
                  ],
                ],
              },
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:44:59.718243Z",
            packageName: "wormhole-core",
            representativePackageId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371",
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 7,
            contractId:
              "00c1657d7245b7ef3456f8f10b27dc04c736dd70687b304d8a34507774c6b6ecdeca12122090abd4dd3c5342698e008538d0b69e591b285cc0aa8eec8fbdb89eeb6800b4ed",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Deposit:DepositPreapproval",
            interfaceId: null,
            choice: "Deposit",
            choiceArgument: {
              amount: "2.5000000000",
              factoryCid:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 9,
            exerciseResult: {},
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 483,
            nodeId: 8,
            contractId:
              "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
            interfaceId:
              "9cc2cbc838ef38dc2c7f34014c9c452bcf71b8e2a4f939235fc0b5d0924b185e:Splice.Api.Token.BurnMintV1:BurnMintFactory",
            choice: "BurnMintFactory_BurnMint",
            choiceArgument: {
              expectedAdmin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              inputHoldingCids: [],
              outputs: [
                {
                  owner:
                    "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  amount: "2.5000000000",
                  context: {
                    values: {},
                  },
                },
              ],
              extraActors: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 9,
            exerciseResult: {
              outputCids: [
                "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
              ],
            },
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 483,
            nodeId: 9,
            contractId:
              "00da8c10476f58fff7138d756dc5277b2d9a99efc0ba62af4c112f309a56ce31edca12122025785937301522ce922f7457d42d744193a1389612e8cecd04d42ca205ef0e90",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              owner:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "2.5000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:44:59.718243Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: true,
          },
        },
      ],
      offset: 483,
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      traceContext: {
        traceparent: "00-fcfd0460311bbe17284a263a6d538fe2-995f08a28bd33ddd-03",
        tracestate: null,
      },
      recordTime: "2026-11-24T22:44:59.765959Z",
      externalTransactionHash: null,
      paidTrafficCost: 0,
      transactionHash: null,
    },
    vaa: "010000000001008ded904ccae57e94833cf1bb59636b1f1872a262819fcc264182033b307a9b0063bb1bab77797c29937ca89c52d1042ed296cbae33ae071bae1d30050aece1a6006b06136b000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d75d5d5cec89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d75d5d5ce00000000000000000000000000000000000000000000000000000000000000ee004f994e545408000000000ee6b28000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
  },
  release: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
            choice: "Release",
            choiceArgument: {
              executor:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              replayNodeCid:
                "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
              vaaBytes: {
                tag: "EncodedVaa",
                value:
                  "01000000000100bc8ec9ec1067526b2c684287ab1c9a502b1ed3afdf120391293c6a568ce81c157b742b9d844b4e7aa0e31e034aaaf827ae4efa2813d249c53374371a1bf43c26006b061372000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7644edb3c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              recipient:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ledgerCid:
                "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
              inputHoldingCids: [],
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
            },
          },
        },
      ],
      commandId:
        "ntt-private-redeem-177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b",
      actAs: [
        "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          createdEventBlob:
            "CgMyLjES2QwKRQBm5hRP6EnBqzGYWKlPAhlLm0ZeooOn9paWx20c0segFcoSEiASOUxFSVoLaKO6lXYlrJHmKph7kLkMJQ4W+MuZUa94yRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIiowlqoAkKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAIKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKBAoCGAIKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNQoQCg56DAoKTG9ja1VubG9jawoECgIYFApaClhyVgoQVW5pdmVyc2FsQWRkcmVzcxJCQkBmODNjZDE0NzY2MjU5NjkwMjJkNDk4ODA3MWJjMWM4Y2VlZTQwYTQwMGY3ZmI4NDMwNmE2MDQ0NmMzZjc0NjQ2CtABCs0BYsoBCscBCgIYBBLAAWq9AQpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYjFmZjVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYzJhYTVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgoECgIYJAphCl9yXQoSVHJhbnNmZXJGYWN0b3J5Q2lkEkdKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuQoECgIQAAoECgIYAAoECgIYAipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgRZ1cBcHlFZ2jh9kHw0+yV/q9ayqiBuSc5vPcTArfjF0QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
          contractId:
            "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
          createdEventBlob:
            "CgMyLjEStwUKRQAIHB5DThpKEWvHw8drqiarJ4YFdezm8JaetWf8PKUrgsoSEiA7+RoJYkT30YQx6B7eprfoH7RfC+s4+XBPEswbZTIa7xINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlItgCatUCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKCgoIagYKBAoCYgAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5XsxAdQ1hBgBCKgomCiQIARIgl8TGJMLeImZsajP701M0UTX5jaSudtEP0402HvlWYtwQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
          createdEventBlob:
            "CgMyLjES8QUKRQCYgeX+s0c9Sjnzh/0zBQpJv6t2Yu/f6ZJ8H7WTuZoNZ8oSEiBIIwM1q/mpCkxJPKGkRJcWbox2/OeteBK+VgKH1CO0SRIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIpsDapgDCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKEAoOMgwxLjUwMDAwMDAwMDAKTQpLUkkKR0pFAF42fLdHTj6EpSRE2QuPYBWcGkA05VMyzt1NVZgvV86kyhISIC2NdRlzc3MINreg9adU3sRnXZy6xVxjfCAv8ylo9FUFKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIDmafL33lg4Gl5t9MCRAbcdoQe7o+QPAodYXU09n2fZxEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
          contractId:
            "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
          createdEventBlob:
            "CgMyLjESuAUKRQBeNny3R04+hKUkRNkLj2AVnBpANOVTMs7dTVWYL1fOpMoSEiAtjXUZc3NzCDa3oPWnVN7EZ12cusVcY3wgL/MpaPRVBRINdG9rZW4tY2lwMDA1NhpeCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIEQ29pbhoEQ29pbiKAAmr9AQqDAQqAAWp+CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoPCg1CC3Nkay1sb2NrLXM1CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKEAoOMgwxLjUwMDAwMDAwMDAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIJeI44vXMOAW2VhLrRFGjOWPPpKHcs2emYuchS3XOE6+EB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
          contractId:
            "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
          createdEventBlob:
            "CgMyLjESsQUKRQCmrhZYgQOv8L/FFHLji6pFoh+DqTFMniKvmefF8ogQS8oSEiBGhKwuHvCiuX9Mybzo/oH+Ry05FCp85eMWGrBFWXz5tBINdG9rZW4tY2lwMDA1Nhp1CkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIMQ29pblRyYW5zZmVyGhNUcmFuc2ZlclByZWFwcHJvdmFsIugBauUBCl0KWzpZU2RrVXNlci1zNS0wNWU4YjEwNzo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNSpZU2RrVXNlci1zNS0wNWU4YjEwNzo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYyZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5oItjdQ1hBgBCKgomCiQIARIgs/BXGTCyy3HVtEVgqwOUlvTLCQPZ3+Yk8NmuhUV7SD4QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    flowUrl:
      "/v1/flows/release?manager=26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2&digest=177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b&recipient=SdkUser-s5-05e8b107%3A%3A122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
    flow: {
      flow: "release",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          createdEventBlob:
            "CgMyLjES2QwKRQBm5hRP6EnBqzGYWKlPAhlLm0ZeooOn9paWx20c0segFcoSEiASOUxFSVoLaKO6lXYlrJHmKph7kLkMJQ4W+MuZUa94yRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIiowlqoAkKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAIKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKBAoCGAIKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNQoQCg56DAoKTG9ja1VubG9jawoECgIYFApaClhyVgoQVW5pdmVyc2FsQWRkcmVzcxJCQkBmODNjZDE0NzY2MjU5NjkwMjJkNDk4ODA3MWJjMWM4Y2VlZTQwYTQwMGY3ZmI4NDMwNmE2MDQ0NmMzZjc0NjQ2CtABCs0BYsoBCscBCgIYBBLAAWq9AQpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYjFmZjVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYzJhYTVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgoECgIYJAphCl9yXQoSVHJhbnNmZXJGYWN0b3J5Q2lkEkdKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuQoECgIQAAoECgIYAAoECgIYAipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgRZ1cBcHlFZ2jh9kHw0+yV/q9ayqiBuSc5vPcTArfjF0QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "1",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
            },
            transceiverEmitterId: "1",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-lock-s5",
            },
            tokenConfig: "LockUnlock",
            tokenDecimals: "10",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f74646",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "TransferFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "covering ReplayNode",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
          contractId:
            "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
          createdEventBlob:
            "CgMyLjEStwUKRQAIHB5DThpKEWvHw8drqiarJ4YFdezm8JaetWf8PKUrgsoSEiA7+RoJYkT30YQx6B7eprfoH7RfC+s4+XBPEswbZTIa7xINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlItgCatUCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKCgoIagYKBAoCYgAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5XsxAdQ1hBgBCKgomCiQIARIgl8TGJMLeImZsajP701M0UTX5jaSudtEP0402HvlWYtwQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            consumer:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            namespace:
              "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
            prefix: {
              tag: "DigestNibblePrefix",
              value: "",
            },
            splitThreshold: "128",
            consumed: {
              map: [],
            },
          },
        },
        {
          role: "LockedLedger",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
          createdEventBlob:
            "CgMyLjES8QUKRQCYgeX+s0c9Sjnzh/0zBQpJv6t2Yu/f6ZJ8H7WTuZoNZ8oSEiBIIwM1q/mpCkxJPKGkRJcWbox2/OeteBK+VgKH1CO0SRIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIpsDapgDCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKEAoOMgwxLjUwMDAwMDAwMDAKTQpLUkkKR0pFAF42fLdHTj6EpSRE2QuPYBWcGkA05VMyzt1NVZgvV86kyhISIC2NdRlzc3MINreg9adU3sRnXZy6xVxjfCAv8ylo9FUFKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIDmafL33lg4Gl5t9MCRAbcdoQe7o+QPAodYXU09n2fZxEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
            },
            balance: "1.5000000000",
            custodyHoldingCid:
              "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
          },
        },
        {
          role: "custody pot Holding",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
          contractId:
            "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
          createdEventBlob:
            "CgMyLjESuAUKRQBeNny3R04+hKUkRNkLj2AVnBpANOVTMs7dTVWYL1fOpMoSEiAtjXUZc3NzCDa3oPWnVN7EZ12cusVcY3wgL/MpaPRVBRINdG9rZW4tY2lwMDA1NhpeCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIEQ29pbhoEQ29pbiKAAmr9AQqDAQqAAWp+CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoPCg1CC3Nkay1sb2NrLXM1CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKEAoOMgwxLjUwMDAwMDAwMDAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIJeI44vXMOAW2VhLrRFGjOWPPpKHcs2emYuchS3XOE6+EB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-lock-s5",
            },
            owner:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            amount: "1.5000000000",
          },
        },
        {
          role: "committed TransferFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
        {
          role: "recipient TransferPreapproval",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
          contractId:
            "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
          createdEventBlob:
            "CgMyLjESsQUKRQCmrhZYgQOv8L/FFHLji6pFoh+DqTFMniKvmefF8ogQS8oSEiBGhKwuHvCiuX9Mybzo/oH+Ry05FCp85eMWGrBFWXz5tBINdG9rZW4tY2lwMDA1Nhp1CkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIMQ29pblRyYW5zZmVyGhNUcmFuc2ZlclByZWFwcHJvdmFsIugBauUBCl0KWzpZU2RrVXNlci1zNS0wNWU4YjEwNzo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNSpZU2RrVXNlci1zNS0wNWU4YjEwNzo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYyZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5oItjdQ1hBgBCKgomCiQIARIgs/BXGTCyy3HVtEVgqwOUlvTLCQPZ3+Yk8NmuhUV7SD4QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            owner:
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-lock-s5",
            },
          },
        },
      ],
      missing: [],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
    holdings: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 487,
              nodeId: 6,
              contractId:
                "008f012e25ef7ce26d0a9f1d3675545c11c1015d35f2db75e95d244cff3298c04dca121220f807ad00951cfce4cc24d20c3328eaa230309c53cfd4aa7a3b3e68d5707507eb",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "1.5000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                    },
                    amount: "1.5000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:45:05.793479Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMI0AUSBGBuWI4=",
      },
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 490,
              nodeId: 6,
              contractId:
                "006cdf2f7e190a2f37b3a3babd6b8948d5d981e46be6f2d65cf3e44bb600590ddcca1212208e5958186b4eef7fed2bc6a25716c63c063e98af3dd361003dfe94f819d9818a",
              templateId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                owner:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "98.5000000000",
              },
              createdEventBlob: "",
              interfaceViews: [
                {
                  interfaceId:
                    "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
                  viewStatus: {
                    code: 0,
                    message: "",
                    details: [],
                  },
                  viewValue: {
                    owner:
                      "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                    instrumentId: {
                      admin:
                        "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                      id: "sdk-lock-s5",
                    },
                    amount: "98.5000000000",
                    lock: null,
                    meta: {
                      values: {},
                    },
                  },
                  implementationPackageId:
                    "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
                },
              ],
              witnessParties: [
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:45:06.585486Z",
              packageName: "token-cip0056",
              representativePackageId:
                "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMI2AUSBGBuWI4=",
      },
    ],
    transaction: {
      updateId:
        "1220607e1070f27cd9056f3c2e31add48c7dd23d1b0e39efe1450e379c3e63f4a38b",
      commandId:
        "ntt-private-redeem-177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b",
      workflowId: "",
      effectiveAt: "2026-11-24T22:45:12.586621Z",
      events: [
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 0,
            contractId:
              "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            interfaceId: null,
            choice: "Release",
            choiceArgument: {
              executor:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              replayNodeCid:
                "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
              vaaBytes: {
                tag: "EncodedVaa",
                value:
                  "01000000000100bc8ec9ec1067526b2c684287ab1c9a502b1ed3afdf120391293c6a568ce81c157b742b9d844b4e7aa0e31e034aaaf827ae4efa2813d249c53374371a1bf43c26006b061372000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7644edb3c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              recipient:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ledgerCid:
                "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
              inputHoldingCids: [],
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 17,
            exerciseResult: {
              _1: "006695ab29e5f90fa950b37f2d6022e87264fa97e41c234ad10313ed44c8df8599ca121220a81517c4b76a5ef15c82806efa5620537f5618e7284c47a39f1b3237b4b4e7d2",
              _2: {
                decimals: "8",
                amount: "50000000",
                sourceToken: {
                  tag: "UniversalAddress",
                  value:
                    "00000000000000000000000000000000000000000000000000000000000000dd",
                },
                recipientAddress: {
                  tag: "UniversalAddress",
                  value:
                    "eaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02",
                },
                recipientChain: "75",
                additionalPayload: {
                  tag: "HexBytes",
                  value: "",
                },
              },
            },
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 2,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "GetGuardianGovernance",
            choiceArgument: {
              reader:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            },
            actingParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 2,
            exerciseResult:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 3,
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            interfaceId: null,
            choice: "VerifyAndConsumeVAA",
            choiceArgument: {
              encodedVAA: {
                tag: "EncodedVaa",
                value:
                  "01000000000100bc8ec9ec1067526b2c684287ab1c9a502b1ed3afdf120391293c6a568ce81c157b742b9d844b4e7aa0e31e034aaaf827ae4efa2813d249c53374371a1bf43c26006b061372000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7644edb3c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
              },
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
              replayNodeCid:
                "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 6,
            exerciseResult: {
              _1: {
                version: "1",
                guardianSetIndex: "0",
                signatures: [
                  {
                    guardianIndex: "0",
                    r: {
                      tag: "EcdsaSignatureR",
                      value:
                        "bc8ec9ec1067526b2c684287ab1c9a502b1ed3afdf120391293c6a568ce81c15",
                    },
                    s: {
                      tag: "EcdsaSignatureS",
                      value:
                        "7b742b9d844b4e7aa0e31e034aaaf827ae4efa2813d249c53374371a1bf43c26",
                    },
                    v: "0",
                  },
                ],
                timestamp: "1795560306",
                nonce: "0",
                emitterChain: "2",
                emitterAddress: {
                  tag: "EmitterAddress",
                  value:
                    "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                },
                sequence: "1795560306961843",
                consistencyLevel: "200",
                payload: {
                  tag: "MessagePayload",
                  value:
                    "9945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
                },
                body: {
                  tag: "HexBytes",
                  value:
                    "6b061372000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7644edb3c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
                },
                hash: {
                  tag: "VaaDigest",
                  value:
                    "177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b",
                },
              },
              _2: "00e65b0a6ba12d896179378a085145101797dae04d781265d45d0c6e3de6549a90ca121220c51cf02a09dada23fb5adf7ac18dac97c03c2e8e4aaf2658906712be740aa4f4",
            },
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 5,
            contractId:
              "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            interfaceId: null,
            choice: "ConsumeDigest",
            choiceArgument: {
              digest: {
                tag: "VaaDigest",
                value:
                  "177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b",
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 6,
            exerciseResult:
              "00e65b0a6ba12d896179378a085145101797dae04d781265d45d0c6e3de6549a90ca121220c51cf02a09dada23fb5adf7ac18dac97c03c2e8e4aaf2658906712be740aa4f4",
            packageName: "wormhole-core",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 493,
            nodeId: 6,
            contractId:
              "00e65b0a6ba12d896179378a085145101797dae04d781265d45d0c6e3de6549a90ca121220c51cf02a09dada23fb5adf7ac18dac97c03c2e8e4aaf2658906712be740aa4f4",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
              prefix: {
                tag: "DigestNibblePrefix",
                value: "",
              },
              splitThreshold: "128",
              consumed: {
                map: [
                  [
                    {
                      tag: "DigestNibbleSuffix",
                      value:
                        "177a7e6906803dce6ca3f7bbd32978899b587c007de84855aa9e8776b9b1d54b",
                    },
                    {},
                  ],
                ],
              },
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:12.586621Z",
            packageName: "wormhole-core",
            representativePackageId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371",
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 9,
            contractId:
              "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
            interfaceId:
              "55ba4deb0ad4662c4168b39859738a0e91388d252286480c7331b3f71a517281:Splice.Api.Token.TransferInstructionV1:TransferFactory",
            choice: "TransferFactory_Transfer",
            choiceArgument: {
              expectedAdmin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              transfer: {
                sender:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                receiver:
                  "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                amount: "0.5000000000",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-lock-s5",
                },
                requestedAt: "2026-11-24T22:45:12.586621Z",
                executeBefore: "2026-11-25T22:45:12.586621Z",
                inputHoldingCids: [
                  "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
                ],
                meta: {
                  values: {},
                },
              },
              extraArgs: {
                context: {
                  values: {
                    "token-cip0056.transferPreapprovalCid": {
                      tag: "AV_ContractId",
                      value:
                        "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
                    },
                  },
                },
                meta: {
                  values: {},
                },
              },
            },
            actingParties: [
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 14,
            exerciseResult: {
              output: {
                tag: "TransferInstructionResult_Completed",
                value: {
                  receiverHoldingCids: [
                    "005d6d0786e581a379b11eaf861f4e553a9f4bd006b896eb5b70771df15a1fafd4ca121220cc56ebcf20cd7b3d3201fca0d8bc1c6810df261db7595d9fabccad8dcefadbc1",
                  ],
                },
              },
              senderChangeCids: [
                "000357da9410d19341fa1cd11f347ea838c827b657c7f4f80921b0b61541e23775ca1212209c292ce5cb4e1c3203bae11d036c8cdab04128fb3e90ff912f3dc528e241fae4",
              ],
              meta: {
                values: {},
              },
            },
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 11,
            contractId:
              "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            interfaceId:
              "718a0f77e505a8de22f188bd4c87fe74101274e9d4cb1bfac7d09aec7158d35b:Splice.Api.Token.HoldingV1:Holding",
            choice: "Archive",
            choiceArgument: {},
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 11,
            exerciseResult: {},
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 493,
            nodeId: 12,
            contractId:
              "000357da9410d19341fa1cd11f347ea838c827b657c7f4f80921b0b61541e23775ca1212209c292ce5cb4e1c3203bae11d036c8cdab04128fb3e90ff912f3dc528e241fae4",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
              owner:
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "1.0000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:12.586621Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: false,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 13,
            contractId:
              "00a6ae16588103aff0bfc51472e38baa45a21f83a9314c9e22af99e7c5f288104bca1212204684ac2e1ef0a2b97f4cc9bce8fe81fe472d39142a7ce5e3161ab045597cf9b4",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinTransfer:TransferPreapproval",
            interfaceId: null,
            choice: "TransferPreapproval_Receive",
            choiceArgument: {
              receiver:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "0.5000000000",
              expectedInstrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: false,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 14,
            exerciseResult:
              "005d6d0786e581a379b11eaf861f4e553a9f4bd006b896eb5b70771df15a1fafd4ca121220cc56ebcf20cd7b3d3201fca0d8bc1c6810df261db7595d9fabccad8dcefadbc1",
            packageName: "token-cip0056",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 493,
            nodeId: 14,
            contractId:
              "005d6d0786e581a379b11eaf861f4e553a9f4bd006b896eb5b70771df15a1fafd4ca121220cc56ebcf20cd7b3d3201fca0d8bc1c6810df261db7595d9fabccad8dcefadbc1",
            templateId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "sdk-lock-s5",
              },
              owner:
                "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              amount: "0.5000000000",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:12.586621Z",
            packageName: "token-cip0056",
            representativePackageId:
              "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474",
            acsDelta: true,
          },
        },
        {
          ExercisedEvent: {
            offset: 493,
            nodeId: 16,
            contractId:
              "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
            interfaceId: null,
            choice: "Debit",
            choiceArgument: {
              amount: "0.5000000000",
              newCustodyHint:
                "000357da9410d19341fa1cd11f347ea838c827b657c7f4f80921b0b61541e23775ca1212209c292ce5cb4e1c3203bae11d036c8cdab04128fb3e90ff912f3dc528e241fae4",
            },
            actingParties: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            consuming: true,
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            lastDescendantNodeId: 17,
            exerciseResult:
              "006695ab29e5f90fa950b37f2d6022e87264fa97e41c234ad10313ed44c8df8599ca121220a81517c4b76a5ef15c82806efa5620537f5618e7284c47a39f1b3237b4b4e7d2",
            packageName: "ntt",
            implementedInterfaces: [],
            acsDelta: false,
          },
        },
        {
          CreatedEvent: {
            offset: 493,
            nodeId: 17,
            contractId:
              "006695ab29e5f90fa950b37f2d6022e87264fa97e41c234ad10313ed44c8df8599ca121220a81517c4b76a5ef15c82806efa5620537f5618e7284c47a39f1b3237b4b4e7d2",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
            contractKey: null,
            contractKeyHash: "",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              managerAddress: {
                tag: "NttManagerAddress",
                value:
                  "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
              },
              balance: "1.0000000000",
              custodyHoldingCid:
                "000357da9410d19341fa1cd11f347ea838c827b657c7f4f80921b0b61541e23775ca1212209c292ce5cb4e1c3203bae11d036c8cdab04128fb3e90ff912f3dc528e241fae4",
            },
            createdEventBlob: "",
            interfaceViews: [],
            witnessParties: [
              "SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            signatories: [
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            ],
            observers: [],
            createdAt: "2026-11-24T22:45:12.586621Z",
            packageName: "ntt",
            representativePackageId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
            acsDelta: false,
          },
        },
      ],
      offset: 493,
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      traceContext: {
        traceparent: "00-33592ddc08c698fa4fe8367af5130932-c4a2293b33ded55d-03",
        tracestate: null,
      },
      recordTime: "2026-11-24T22:45:12.627662Z",
      externalTransactionHash: null,
      paidTrafficCost: 0,
      transactionHash: null,
    },
    vaa: "01000000000100bc8ec9ec1067526b2c684287ab1c9a502b1ed3afdf120391293c6a568ce81c157b742b9d844b4e7aa0e31e034aaaf827ae4efa2813d249c53374371a1bf43c26006b061372000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7644edb3c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f200910000000000000000000000000000000000000000000000000006610d7644edb300000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000002faf08000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
  },
  releaseMissingPreapproval: {
    flow: "release",
    disclosures: [
      {
        role: "NttManager",
        templateId:
          "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
        contractId:
          "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
        createdEventBlob:
          "CgMyLjES2QwKRQBm5hRP6EnBqzGYWKlPAhlLm0ZeooOn9paWx20c0segFcoSEiASOUxFSVoLaKO6lXYlrJHmKph7kLkMJQ4W+MuZUa94yRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIiowlqoAkKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAIKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKBAoCGAIKgwEKgAFqfgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKDwoNQgtzZGstbG9jay1zNQoQCg56DAoKTG9ja1VubG9jawoECgIYFApaClhyVgoQVW5pdmVyc2FsQWRkcmVzcxJCQkBmODNjZDE0NzY2MjU5NjkwMjJkNDk4ODA3MWJjMWM4Y2VlZTQwYTQwMGY3ZmI4NDMwNmE2MDQ0NmMzZjc0NjQ2CtABCs0BYsoBCscBCgIYBBLAAWq9AQpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYjFmZjVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgpYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwYzJhYTVlOGIyYzFmM2E2ZDBhOWIxYzJkM2U0ZjVhNmI3YzhkOWUwZgoECgIYJAphCl9yXQoSVHJhbnNmZXJGYWN0b3J5Q2lkEkdKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuQoECgIQAAoECgIYAAoECgIYAipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5HjNMdQ1hBgBCKgomCiQIARIgRZ1cBcHlFZ2jh9kHw0+yV/q9ayqiBuSc5vPcTArfjF0QHg==",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          namespace:
            "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          guardianGovernance:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          nttCustody:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          chainId: "75",
          managerId: "1",
          managerAddress: {
            tag: "NttManagerAddress",
            value:
              "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          },
          transceiverEmitterId: "1",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          tokenConfig: "LockUnlock",
          tokenDecimals: "10",
          tokenAddress: {
            tag: "UniversalAddress",
            value:
              "f83cd1476625969022d4988071bc1c8ceee40a400f7fb84306a60446c3f74646",
          },
          peers: [
            [
              "2",
              {
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                },
                transceiverAddress: {
                  tag: "EmitterAddress",
                  value:
                    "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                },
                decimals: "18",
              },
            ],
          ],
          factory: {
            tag: "TransferFactoryCid",
            value:
              "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          },
          paused: false,
          factoryEpoch: "0",
          peerEpoch: "1",
        },
      },
      {
        role: "CoreState",
        templateId:
          "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
        contractId:
          "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
        createdEventBlob:
          "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          guardianGovernance:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          chainId: "75",
          governanceChainId: "1",
          governanceContract: {
            tag: "EmitterAddress",
            value:
              "0000000000000000000000000000000000000000000000000000000000000004",
          },
          guardianSetIndex: "0",
          guardianSets: [
            [
              "0",
              {
                members: [
                  {
                    address: {
                      tag: "GuardianAddress",
                      value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                    },
                    publicKey: {
                      tag: "GuardianPublicKey",
                      value:
                        "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                    },
                  },
                ],
              },
            ],
          ],
          messageFee: "0",
          feeInstrument: {
            admin:
              "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "UNUSED",
          },
          feeRecipient:
            "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          consumedGovernance: [],
        },
      },
      {
        role: "covering ReplayNode",
        templateId:
          "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
        contractId:
          "00081c1e434e1a4a116bc7c3c76baa26ab27860575ece6f0969eb567fc3ca52b82ca1212203bf91a096244f7d18431e81edea6b7e81fb45f0beb38f9704f12cc1b65321aef",
        createdEventBlob:
          "CgMyLjEStwUKRQAIHB5DThpKEWvHw8drqiarJ4YFdezm8JaetWf8PKUrgsoSEiA7+RoJYkT30YQx6B7eprfoH7RfC+s4+XBPEswbZTIa7xINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlItgCatUCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZmMwNmNmOTRlZjY0ZjA0NDUzYzYyZWQwMDNmM2M1ZjRkZDlkMzFmMjZlOWQ1ZDliYjA3MDkzN2I2ODQ3YjZlZAocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKCgoIagYKBAoCYgAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5XsxAdQ1hBgBCKgomCiQIARIgl8TGJMLeImZsajP701M0UTX5jaSudtEP0402HvlWYtwQHg==",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          consumer:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          guardianGovernance:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          namespace:
            "ntt:fc06cf94ef64f04453c62ed003f3c5f4dd9d31f26e9d5d9bb070937b6847b6ed",
          prefix: {
            tag: "DigestNibblePrefix",
            value: "",
          },
          splitThreshold: "128",
          consumed: {
            map: [],
          },
        },
      },
      {
        role: "LockedLedger",
        templateId:
          "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
        contractId:
          "009881e5feb3473d4a39f387fd33050a49bfab7662efdfe9927c1fb593b99a0d67ca12122048230335abf9a90a4c493ca1a44497166e8c76fce7ad7812be560287d423b449",
        createdEventBlob:
          "CgMyLjES8QUKRQCYgeX+s0c9Sjnzh/0zBQpJv6t2Yu/f6ZJ8H7WTuZoNZ8oSEiBIIwM1q/mpCkxJPKGkRJcWbox2/OeteBK+VgKH1CO0SRIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIpsDapgDCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDI2ZjUwYTU0NDJiZGEwNGZkZGU3ODc4MmViYjg1ZGFjY2VhNGQ3MjY1NjA4ZWQxNmYyMmQxN2MyOWQ2ZWE2ZjIKEAoOMgwxLjUwMDAwMDAwMDAKTQpLUkkKR0pFAF42fLdHTj6EpSRE2QuPYBWcGkA05VMyzt1NVZgvV86kyhISIC2NdRlzc3MINreg9adU3sRnXZy6xVxjfCAv8ylo9FUFKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIDmafL33lg4Gl5t9MCRAbcdoQe7o+QPAodYXU09n2fZxEB4=",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          guardianGovernance:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          managerAddress: {
            tag: "NttManagerAddress",
            value:
              "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          },
          balance: "1.5000000000",
          custodyHoldingCid:
            "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
        },
      },
      {
        role: "custody pot Holding",
        templateId:
          "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.Coin:Coin",
        contractId:
          "005e367cb7474e3e84a52444d90b8f60159c1a4034e55332cedd4d55982f57cea4ca1212202d8d75197373730836b7a0f5a754dec4675d9cbac55c637c202ff32968f45505",
        createdEventBlob:
          "CgMyLjESuAUKRQBeNny3R04+hKUkRNkLj2AVnBpANOVTMs7dTVWYL1fOpMoSEiAtjXUZc3NzCDa3oPWnVN7EZ12cusVcY3wgL/MpaPRVBRINdG9rZW4tY2lwMDA1NhpeCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhIEQ29pbhoEQ29pbiKAAmr9AQqDAQqAAWp+CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoPCg1CC3Nkay1sb2NrLXM1CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKEAoOMgwxLjUwMDAwMDAwMDAqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OY4vP3YNYQYAQioKJgokCAESIJeI44vXMOAW2VhLrRFGjOWPPpKHcs2emYuchS3XOE6+EB4=",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          owner:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          amount: "1.5000000000",
        },
      },
      {
        role: "committed TransferFactory",
        templateId:
          "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
        contractId:
          "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
        createdEventBlob:
          "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
        synchronizerId:
          "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        createArgument: {
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
        },
      },
    ],
    missing: [
      {
        role: "recipient TransferPreapproval",
        reason: "no matching contract visible to the reader parties",
      },
    ],
  },
  mintNotFound: {
    error:
      "disclosure-service: no DepositPreapproval found for recipient SdkOther-s5-2d0cd0f1::122000f24d... and this deployment's instrument",
    requestId: "824af5c73b585f8146d96284d6c33ff8",
  },
  staleRejection: {
    status: 404,
    body: {
      code: "CONTRACT_NOT_FOUND",
      cause:
        "Contract could not be found with id 005d6d0786e581a379b11eaf861f4e553a9f4bd006b896eb5b70771df15a1fafd4ca121220cc56ebcf20cd7b3d3201fca0d8bc1c6810df261db7595d9fabccad8dcefadbc1",
      correlationId: "118036da-49dd-41bd-9a31-16bf58f42fea",
      traceId: "0925ae8ad44ad7d4e12a16b4105fb2a6",
      context: {
        participant: "'sandbox'",
        category: "11",
        tid: "0925ae8ad44ad7d4e12a16b4105fb2a6",
        definite_answer: "false",
        commands:
          "{readAs: [], deduplicationPeriod: {duration: 'PT168H'}, submittedAt: '2026-11-24T22:45:18.931746Z', submissionId: '118036da-49dd-41bd-9a31-16bf58f42fea', actAs: ['SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6'], commandId: 'ntt-private-send-5953cbf428d00d4af1764dc9af3c3c7e4f369c87e6b6ee9292620e7a3915a602', userId: 'ntt-sdk-test', workflowId: }",
      },
      resources: [
        [
          "CONTRACT_ID",
          "005d6d0786e581a379b11eaf861f4e553a9f4bd006b896eb5b70771df15a1fafd4ca121220cc56ebcf20cd7b3d3201fca0d8bc1c6810df261db7595d9fabccad8dcefadbc1",
        ],
      ],
      errorCategory: 11,
      grpcCodeValue: 5,
      retryInfo: null,
      definiteAnswer: null,
    },
  },
  mintDuplicateError:
    '{"code":"DUPLICATE_COMMAND","cause":"Command submission already exists.","correlationId":null,"traceId":"160f4b86ce18dfd0558e792b90fab50b","context":{"participant":"sandbox","changeId":"ChangeId(ntt-sdk-test,ntt-private-redeem-77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc,Set(SdkUser-s5-05e8b107::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6))","completion_offset":"483","accepted":"true","psid":"synchronizer-1::12201fd1b2a0...::37-0","category":"10","tid":"160f4b86ce18dfd0558e792b90fab50b","definite_answer":"true","existingSubmissionId":"Some(778f3fa7-ba84-4dd1-8334-5cefe599d3a7)"},"resources":[],"errorCategory":10,"grpcCodeValue":6,"retryInfo":null,"definiteAnswer":null}',
  mintReplayError:
    '{"code":"DAML_FAILURE","cause":"Interpretation error: Error: User failure: UNHANDLED_EXCEPTION/DA.Exception.AssertionFailed:AssertionFailed (error category 9): digest already consumed","correlationId":"bdb9daff-c11a-4905-a7b8-16453cabbf34","traceId":"0e80adaf96ad3e2aeda8bcfb664b4e88","context":{"participant":"\'sandbox\'","error_id":"UNHANDLED_EXCEPTION/DA.Exception.AssertionFailed:AssertionFailed","exercise_trace":"    in choice 32abbcd2:Wormhole.Core.Replay:ReplayNode:ConsumeDigest on contract 0082432374 (#4)\\n    in choice 32abbcd2:Wormhole.Core.State:CoreState:VerifyAndConsumeVAA on contract 001f8894b6 (#2)\\n    in choice 43e1b206:Wormhole.Ntt.Manager:NttManager:Mint on contract 00ef6beff2 (#0)\\n    in exercise command 43e1b206:Wormhole.Ntt.Manager:NttManager:Mint on contract 00ef6beff2.\\n","category":"9","tid":"0e80adaf96ad3e2aeda8bcfb664b4e88","definite_answer":"false","commands":"{readAs: [], deduplicationPeriod: {duration: \'PT168H\'}, submittedAt: \'2026-11-24T22:45:05.726345Z\', submissionId: \'bdb9daff-c11a-4905-a7b8-16453cabbf34\', actAs: [\'SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6\'], commandId: \'ntt-private-redeem-77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc\', userId: \'ntt-sdk-test\', workflowId: }"},"resources":[],"errorCategory":9,"grpcCodeValue":9,"retryInfo":null,"definiteAnswer":null}',
  executed: {
    vaa: "010000000001009dfdf1734f408fcfdcc331f6c39b6f26ba5551ae5d595480b6f57f0d9152c61d7e627abd70952f739b5e4cbc8e4e600357f20fa8f290ff45ca746f5c51a24fb4016b061384000000000002000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f0006610d7755cf68c89945ff10000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c51338200910000000000000000000000000000000000000000000000000006610d7755cf6800000000000000000000000000000000000000000000000000000000000000ee004f994e5454080000000005f5e10000000000000000000000000000000000000000000000000000000000000000ddeaeada48e4d91dcffb594fc084182baa062e20f0c368d445ee2bcf19a8e7ba02004b0000",
    before: {
      flowUrl:
        "/v1/flows/set-peer?manager=00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382&digest=cacaf2b656647eb9a6d48e7028171e5501244aab8319ebb52aa5b8f4a9268c23",
      flow: {
        flow: "set-peer",
        disclosures: [
          {
            role: "NttManager",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            createdEventBlob:
              "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              admin:
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              nttCustody:
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              reserveCustodian:
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              chainId: "75",
              managerId: "0",
              managerAddress: {
                tag: "NttManagerAddress",
                value:
                  "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
              },
              transceiverEmitterId: "0",
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              tokenConfig: "BurnMint",
              tokenDecimals: "8",
              tokenAddress: {
                tag: "UniversalAddress",
                value:
                  "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
              },
              peers: [
                [
                  "2",
                  {
                    managerAddress: {
                      tag: "NttManagerAddress",
                      value:
                        "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                    },
                    transceiverAddress: {
                      tag: "EmitterAddress",
                      value:
                        "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                    },
                    decimals: "18",
                  },
                ],
              ],
              factory: {
                tag: "BurnMintFactoryCid",
                value:
                  "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
              },
              paused: false,
              factoryEpoch: "0",
              peerEpoch: "1",
            },
          },
          {
            role: "CoreState",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            createdEventBlob:
              "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              chainId: "75",
              governanceChainId: "1",
              governanceContract: {
                tag: "EmitterAddress",
                value:
                  "0000000000000000000000000000000000000000000000000000000000000004",
              },
              guardianSetIndex: "0",
              guardianSets: [
                [
                  "0",
                  {
                    members: [
                      {
                        address: {
                          tag: "GuardianAddress",
                          value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                        },
                        publicKey: {
                          tag: "GuardianPublicKey",
                          value:
                            "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                        },
                      },
                    ],
                  },
                ],
              ],
              messageFee: "0",
              feeInstrument: {
                admin:
                  "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "UNUSED",
              },
              feeRecipient:
                "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              consumedGovernance: [],
            },
          },
          {
            role: "transceiver Emitter",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            contractId:
              "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
            createdEventBlob:
              "CgMyLjESngUKRQCdxcWs0hDyyK8Gyi1FuD4WTYMq+e1IkwJng/EqucyijMoSEiAr3Jq3LPXOIRaYDMkqmTrOM4N0IrcgxCqs9+S+MttdNhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnHGTN2DWEGAEIqCiYKJAgBEiCIyUUVV9h20h3atDDiKC21oFxqDfOMba116ovJkCzB3RAe",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "0",
              emitterAddress: {
                tag: "EmitterAddress",
                value:
                  "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
              },
              sequence: "3",
            },
          },
          {
            role: "covering ReplayNode",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            contractId:
              "008243237430a5a3bd0335546b3034c8c68a463c4e1213cadf4d192c20cfc3b647ca1212207c1a05f851a14b661e33df6a0e50992b803e95fd4fe4cda0010b5318c5b4ef63",
            createdEventBlob:
              "CgMyLjESmQYKRQCCQyN0MKWjvQM1VGswNMjGikY8ThITyt9NGSwgz8O2R8oSEiB8GgX4UaFLZh4z32oOUJkrgD6V/U/kzaABC1MYxbTvYxINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlIroDarcDCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIKbApqamgKZgpkYmIKYApaclgKEkRpZ2VzdE5pYmJsZVN1ZmZpeBJCQkA3NzU3MDgyOGUyNWM1NGMxYzA5MGQ3NzVlODVkMmE1NTQ3YzA4NWFkOGFmMzZjNWRlY2MzMDk0OGZjNTFkMGRjEgIKACpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjljZtZ1DWEGAEIqCiYKJAgBEiAHCRzSlX0yhW6ckGW0ysLW8+FyXrjnylG56XDdMK3UrRAe",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              prefix: {
                tag: "DigestNibblePrefix",
                value: "",
              },
              splitThreshold: "128",
              consumed: {
                map: [
                  [
                    {
                      tag: "DigestNibbleSuffix",
                      value:
                        "77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
                    },
                    {},
                  ],
                ],
              },
            },
          },
        ],
        missing: [],
      },
    },
    after: {
      flowUrl:
        "/v1/flows/set-peer?manager=00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382&digest=cacaf2b656647eb9a6d48e7028171e5501244aab8319ebb52aa5b8f4a9268c23",
      flow: {
        flow: "set-peer",
        disclosures: [
          {
            role: "NttManager",
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            createdEventBlob:
              "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              admin:
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              nttCustody:
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              reserveCustodian:
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              chainId: "75",
              managerId: "0",
              managerAddress: {
                tag: "NttManagerAddress",
                value:
                  "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
              },
              transceiverEmitterId: "0",
              instrumentId: {
                admin:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
              },
              tokenConfig: "BurnMint",
              tokenDecimals: "8",
              tokenAddress: {
                tag: "UniversalAddress",
                value:
                  "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
              },
              peers: [
                [
                  "2",
                  {
                    managerAddress: {
                      tag: "NttManagerAddress",
                      value:
                        "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                    },
                    transceiverAddress: {
                      tag: "EmitterAddress",
                      value:
                        "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                    },
                    decimals: "18",
                  },
                ],
              ],
              factory: {
                tag: "BurnMintFactoryCid",
                value:
                  "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
              },
              paused: false,
              factoryEpoch: "0",
              peerEpoch: "1",
            },
          },
          {
            role: "CoreState",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
            contractId:
              "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
            createdEventBlob:
              "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              chainId: "75",
              governanceChainId: "1",
              governanceContract: {
                tag: "EmitterAddress",
                value:
                  "0000000000000000000000000000000000000000000000000000000000000004",
              },
              guardianSetIndex: "0",
              guardianSets: [
                [
                  "0",
                  {
                    members: [
                      {
                        address: {
                          tag: "GuardianAddress",
                          value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                        },
                        publicKey: {
                          tag: "GuardianPublicKey",
                          value:
                            "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                        },
                      },
                    ],
                  },
                ],
              ],
              messageFee: "0",
              feeInstrument: {
                admin:
                  "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                id: "UNUSED",
              },
              feeRecipient:
                "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              consumedGovernance: [],
            },
          },
          {
            role: "transceiver Emitter",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
            contractId:
              "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
            createdEventBlob:
              "CgMyLjESngUKRQCdxcWs0hDyyK8Gyi1FuD4WTYMq+e1IkwJng/EqucyijMoSEiAr3Jq3LPXOIRaYDMkqmTrOM4N0IrcgxCqs9+S+MttdNhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnHGTN2DWEGAEIqCiYKJAgBEiCIyUUVV9h20h3atDDiKC21oFxqDfOMba116ovJkCzB3RAe",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              owner:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              emitterId: "0",
              emitterAddress: {
                tag: "EmitterAddress",
                value:
                  "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
              },
              sequence: "3",
            },
          },
          {
            role: "covering ReplayNode",
            templateId:
              "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.Replay:ReplayNode",
            contractId:
              "0021e4198c5dbbe9e6288acc4bd509808721c7eaced2998ad0e13826b1f03aae5aca121220167797c7c3179353c8a541a9efcfd6ce96aebb5b06efc0a93e7ed2a95cdfad1e",
            createdEventBlob:
              "CgMyLjESgQcKRQAh5BmMXbvp5iiKzEvVCYCHIcfqztKZitDhOCax8DquWsoSEiAWd5fHwxeTU8ilQanvz9bOlq67WwbvwKk+ftKpXN+tHhINd29ybWhvbGUtY29yZRpmCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIGUmVwbGF5GgpSZXBsYXlOb2RlIqIEap8ECmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwocChpyGAoSRGlnZXN0TmliYmxlUHJlZml4EgJCAAoFCgMYgAIK0wEK0AFqzQEKygEKxwFixAEKYApaclgKEkRpZ2VzdE5pYmJsZVN1ZmZpeBJCQkA3NzU3MDgyOGUyNWM1NGMxYzA5MGQ3NzVlODVkMmE1NTQ3YzA4NWFkOGFmMzZjNWRlY2MzMDk0OGZjNTFkMGRjEgIKAApgClpyWAoSRGlnZXN0TmliYmxlU3VmZml4EkJCQGNhY2FmMmI2NTY2NDdlYjlhNmQ0OGU3MDI4MTcxZTU1MDEyNDRhYWI4MzE5ZWJiNTJhYTViOGY0YTkyNjhjMjMSAgoAKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OdwVVncNYQYAQioKJgokCAESIDf8OjRpcbXbPsXroQRzCsXkI7Hdl049t1sDPhKH+//GEB4=",
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            createArgument: {
              consumer:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              guardianGovernance:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              namespace:
                "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
              prefix: {
                tag: "DigestNibblePrefix",
                value: "",
              },
              splitThreshold: "128",
              consumed: {
                map: [
                  [
                    {
                      tag: "DigestNibbleSuffix",
                      value:
                        "77570828e25c54c1c090d775e85d2a5547c085ad8af36c5decc30948fc51d0dc",
                    },
                    {},
                  ],
                  [
                    {
                      tag: "DigestNibbleSuffix",
                      value:
                        "cacaf2b656647eb9a6d48e7028171e5501244aab8319ebb52aa5b8f4a9268c23",
                    },
                    {},
                  ],
                ],
              },
            },
          },
        ],
        missing: [],
      },
    },
  },
  pause: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
            choice: "SetPaused",
            choiceArgument: {
              newPaused: true,
            },
          },
        },
      ],
      commandId:
        "wormhole-sdk-a84c90da8b5a43c3585015ebd4f724e7604a0a0cf33eeefc45e0672cc14a858b",
      actAs: [
        "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    acs: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 453,
              nodeId: 1,
              contractId:
                "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
              templateId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                namespace:
                  "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
                admin:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                guardianGovernance:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                nttCustody:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                reserveCustodian:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                chainId: "75",
                managerId: "0",
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
                },
                transceiverEmitterId: "0",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                tokenConfig: "BurnMint",
                tokenDecimals: "8",
                tokenAddress: {
                  tag: "UniversalAddress",
                  value:
                    "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
                },
                peers: [
                  [
                    "2",
                    {
                      managerAddress: {
                        tag: "NttManagerAddress",
                        value:
                          "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      transceiverAddress: {
                        tag: "EmitterAddress",
                        value:
                          "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      decimals: "18",
                    },
                  ],
                ],
                factory: {
                  tag: "BurnMintFactoryCid",
                  value:
                    "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
                },
                paused: false,
                factoryEpoch: "0",
                peerEpoch: "1",
              },
              createdEventBlob:
                "CgMyLjES8A0KRQDva+/y00uvSBzs7G3OFhCNOInvYdIeAqJsrz7DPAE9a8oSEiD+qk/w8j/mDd21o5pSRGe1JNJUNRrtt+ud12/tcmHEbhIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ORZmOXUNYQYAQioKJgokCAESILoiYTgwqVuxoJPipk/a8R/RXahoN/ZtjSj0GvEWV2bqEB4=",
              interfaceViews: [],
              witnessParties: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:49.429014Z",
              packageName: "ntt",
              representativePackageId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIjQUSBB9ayWA=",
      },
    ],
  },
  unpause: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "003459395b8f020d31ddd79aa3b7527f43041452f46d9a02d1d7fb4c510f597ffbca12122077e69e5fa86d18321422c504f1941652a5558fe30dd832a1afa621769290dced",
            choice: "SetPaused",
            choiceArgument: {
              newPaused: false,
            },
          },
        },
      ],
      commandId:
        "wormhole-sdk-1a081644f0bf8df9c1b52bd318cb9122b98b3dccf8d59ae52bbf566d78b229d6",
      actAs: [
        "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    acs: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 505,
              nodeId: 1,
              contractId:
                "003459395b8f020d31ddd79aa3b7527f43041452f46d9a02d1d7fb4c510f597ffbca12122077e69e5fa86d18321422c504f1941652a5558fe30dd832a1afa621769290dced",
              templateId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                namespace:
                  "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
                admin:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                guardianGovernance:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                nttCustody:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                reserveCustodian:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                chainId: "75",
                managerId: "0",
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
                },
                transceiverEmitterId: "0",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                tokenConfig: "BurnMint",
                tokenDecimals: "8",
                tokenAddress: {
                  tag: "UniversalAddress",
                  value:
                    "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
                },
                peers: [
                  [
                    "2",
                    {
                      managerAddress: {
                        tag: "NttManagerAddress",
                        value:
                          "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      transceiverAddress: {
                        tag: "EmitterAddress",
                        value:
                          "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      decimals: "18",
                    },
                  ],
                ],
                factory: {
                  tag: "BurnMintFactoryCid",
                  value:
                    "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
                },
                paused: true,
                factoryEpoch: "0",
                peerEpoch: "1",
              },
              createdEventBlob:
                "CgMyLjES8A0KRQA0WTlbjwINMd3XmqO3Un9DBBRS9G2aAtHX+0xRD1l/+8oSEiB35p5fqG0YMhQixQTxlBZSpVWP4w3YMqGvpiF2kpDc7RIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAEKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OfLTr3cNYQYAQioKJgokCAESIE/FIRkpRLSZlI79dhLq5OzWpDp3WTcUI4xjNBbTw3JTEB4=",
              interfaceViews: [],
              witnessParties: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:45:30.744818Z",
              packageName: "ntt",
              representativePackageId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIlwYSBNR42Ek=",
      },
    ],
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00ef6beff2d34baf481cecec6dce16108d3889ef61d21e02a26caf3ec33c013d6bca121220feaa4ff0f23fe60dddb5a39a524467b524d254351aedb7eb9dd76fed7261c46e",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
  },
  setPeerFull: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
            contractId:
              "00492d8000490fbbef2fb28d0944d112b64bc150036f0f532a08a4fdd2260ce167ca12122074c63d8164339a0c717c45e6dc8cbe56c1f63e3de8e7c1231b9418255c9f94d8",
            choice: "SetPeer",
            choiceArgument: {
              chainId: "21",
              peer: {
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a",
                },
                transceiverAddress: {
                  tag: "EmitterAddress",
                  value:
                    "5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b5b",
                },
                decimals: "9",
              },
              transceiverEmitterCid:
                "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
              coreStateCid:
                "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
              consistencyLevel: "0",
              payer:
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              feeAllocation: null,
            },
          },
        },
      ],
      commandId:
        "wormhole-sdk-4bf302bac6ad6a501df166b2e743ff63782b1925b62586016234a301a40ddd77",
      actAs: [
        "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
          createdEventBlob:
            "CgMyLjESngUKRQCdxcWs0hDyyK8Gyi1FuD4WTYMq+e1IkwJng/EqucyijMoSEiAr3Jq3LPXOIRaYDMkqmTrOM4N0IrcgxCqs9+S+MttdNhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnHGTN2DWEGAEIqCiYKJAgBEiCIyUUVV9h20h3atDDiKC21oFxqDfOMba116ovJkCzB3RAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    acs: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 508,
              nodeId: 1,
              contractId:
                "00492d8000490fbbef2fb28d0944d112b64bc150036f0f532a08a4fdd2260ce167ca12122074c63d8164339a0c717c45e6dc8cbe56c1f63e3de8e7c1231b9418255c9f94d8",
              templateId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                namespace:
                  "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
                admin:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                guardianGovernance:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                nttCustody:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                reserveCustodian:
                  "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                chainId: "75",
                managerId: "0",
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
                },
                transceiverEmitterId: "0",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
                },
                tokenConfig: "BurnMint",
                tokenDecimals: "8",
                tokenAddress: {
                  tag: "UniversalAddress",
                  value:
                    "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
                },
                peers: [
                  [
                    "2",
                    {
                      managerAddress: {
                        tag: "NttManagerAddress",
                        value:
                          "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      transceiverAddress: {
                        tag: "EmitterAddress",
                        value:
                          "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      decimals: "18",
                    },
                  ],
                ],
                factory: {
                  tag: "BurnMintFactoryCid",
                  value:
                    "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
                },
                paused: false,
                factoryEpoch: "0",
                peerEpoch: "1",
              },
              createdEventBlob:
                "CgMyLjES8A0KRQBJLYAASQ+77y+yjQlE0RK2S8FQA28PUyoIpP3SJgzhZ8oSEiB0xj2BZDOaDHF8RebcjL5WwfY+PejnwSMblBglXJ+U2BIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OafUCngNYQYAQioKJgokCAESIKx8/kKVRU0HFLiqw5ATkLSaO7ffggq47t5jyFhEjg0LEB4=",
              interfaceViews: [],
              witnessParties: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:45:36.708775Z",
              packageName: "ntt",
              representativePackageId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMImQYSBJadi/c=",
      },
    ],
    flowUrl:
      "/v1/flows/transfer?manager=00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
    flow: {
      flow: "transfer",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00492d8000490fbbef2fb28d0944d112b64bc150036f0f532a08a4fdd2260ce167ca12122074c63d8164339a0c717c45e6dc8cbe56c1f63e3de8e7c1231b9418255c9f94d8",
          createdEventBlob:
            "CgMyLjES8A0KRQBJLYAASQ+77y+yjQlE0RK2S8FQA28PUyoIpP3SJgzhZ8oSEiB0xj2BZDOaDHF8RebcjL5WwfY+PejnwSMblBglXJ+U2BIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIi2glq1wkKSApGQkRudHQ6ZTAyNjA1OGJiYmIxZWFhZTNjYWE5MWRlNTBlZjNjYmM3MjFiNGYwNTI1NjQ2YTRhZjQyMTMwZWRhNGJjZmUyYwpiCmA6XlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrTnR0Q3VzdG9keS1zNS1jODEyYTc2ZDo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYgpgOl5TZGtCdXJuQWRtaW4tczUtNDBiNWYwNGI6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CgUKAxiWAQoECgIYAApbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAMDA0NjkyMTdmZmIxZmI4YTVhNmQ4NzQ2MThjNjkwYzEyMjIyODQ4YTY3ZWQyODIwZTJmNThiNWEzYzUxMzM4MgoECgIYAArGAQrDAWrAAQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKUQpPQk13b3JtaG9sZS1udHQ6N2I2ZTA1ODM0OTY0NzU0OTAxOTBmZWY5MjdkNDJiMTczODgwZDEyODEzZGU4ZDA3MjE4YzQwOTFlMTU2NTVhOAoOCgx6CgoIQnVybk1pbnQKBAoCGBAKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAMmI2NmVkZGU2YWExNWU4MTQ0YjI4NWQ5MmUwOWJlZjg4Y2Y1YzM1NzU3NTAwNDA4N2ZiMmI3ZGY2ZDZkODA5MgrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KEkJ1cm5NaW50RmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqXlNka0J1cm5BZG1pbi1zNS00MGI1ZjA0Yjo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OafUCngNYQYAQioKJgokCAESIKx8/kKVRU0HFLiqw5ATkLSaO7ffggq47t5jyFhEjg0LEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:e026058bbbb1eaae3caa91de50ef3cbc721b4f0525646a4af42130eda4bcfe2c",
            admin:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "0",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
            },
            transceiverEmitterId: "0",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
            },
            tokenConfig: "BurnMint",
            tokenDecimals: "8",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "2b66edde6aa15e8144b285d92e09bef88cf5c357575004087fb2b7df6d6d8092",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "BurnMintFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "transceiver Emitter",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "009dc5c5acd210f2c8af06ca2d45b83e164d832af9ed4893026783f12ab9cca28cca1212202bdc9ab72cf5ce2116980cc92a993ace33837422b720c42aacf7e4be32db5d36",
          createdEventBlob:
            "CgMyLjESngUKRQCdxcWs0hDyyK8Gyi1FuD4WTYMq+e1IkwJng/EqucyijMoSEiAr3Jq3LPXOIRaYDMkqmTrOM4N0IrcgxCqs9+S+MttdNhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYAApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJAZTQwZjE1MjlmNmQ1NWZlOTgzYjQwOTYwNTRiMjk0NmE2NzBmMmI4NDA2MGEyYmI4Yjk5NWRkNmRhZWQxOWRkMgoECgIYBipnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnHGTN2DWEGAEIqCiYKJAgBEiCIyUUVV9h20h3atDDiKC21oFxqDfOMba116ovJkCzB3RAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            owner:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            emitterId: "0",
            emitterAddress: {
              tag: "EmitterAddress",
              value:
                "e40f1529f6d55fe983b4096054b2946a670f2b84060a2bb8b995dd6daed19dd2",
            },
            sequence: "3",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "committed canonical CoinFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
      ],
      missing: [],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "00492d8000490fbbef2fb28d0944d112b64bc150036f0f532a08a4fdd2260ce167ca12122074c63d8164339a0c717c45e6dc8cbe56c1f63e3de8e7c1231b9418255c9f94d8",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
  },
  setOwner: {
    envelope: {
      commands: [
        {
          CreateCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:AdminTransferProposal",
            createArguments: {
              admin:
                "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              newAdmin:
                "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              managerAddress: {
                tag: "NttManagerAddress",
                value:
                  "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
              },
            },
          },
        },
      ],
      commandId:
        "ntt-admin-propose-9df310a99e549f2c4ae1793c99ea864e83e0802e9f0b1ef1debf09da5ce97910",
      actAs: [
        "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    acs: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 468,
              nodeId: 1,
              contractId:
                "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
              templateId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                namespace:
                  "ntt:569144fba01631def89877535c463e13d579533887f7423fdc3c67cdbe78e187",
                admin:
                  "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                guardianGovernance:
                  "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                nttCustody:
                  "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                reserveCustodian:
                  "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                chainId: "75",
                managerId: "2",
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
                },
                transceiverEmitterId: "2",
                instrumentId: {
                  admin:
                    "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                  id: "sdk-owned-s5",
                },
                tokenConfig: "LockUnlock",
                tokenDecimals: "10",
                tokenAddress: {
                  tag: "UniversalAddress",
                  value:
                    "c6e203c9966adc410fc63e22ad8e47a8a32ab0311ebc18b546710def11a85f3c",
                },
                peers: [
                  [
                    "2",
                    {
                      managerAddress: {
                        tag: "NttManagerAddress",
                        value:
                          "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      transceiverAddress: {
                        tag: "EmitterAddress",
                        value:
                          "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                      },
                      decimals: "18",
                    },
                  ],
                ],
                factory: {
                  tag: "TransferFactoryCid",
                  value:
                    "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
                },
                paused: false,
                factoryEpoch: "0",
                peerEpoch: "1",
              },
              createdEventBlob:
                "CgMyLjESsw0KRQCqrDVl69laudZHP2y37p6axVDu47VcyYhDhDqM0eCEncoSEiC/R/37GOTlyYRvT6RBCFRfQKzgKtjGmtFeCdxdTuI2HRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIinAlqmQkKSApGQkRudHQ6NTY5MTQ0ZmJhMDE2MzFkZWY4OTg3NzUzNWM0NjNlMTNkNTc5NTMzODg3Zjc0MjNmZGMzYzY3Y2RiZTc4ZTE4NwpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpjCmE6X1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgEClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjCgQKAhgECoQBCoEBan8KawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ChAKDkIMc2RrLW93bmVkLXM1ChAKDnoMCgpMb2NrVW5sb2NrCgQKAhgUCloKWHJWChBVbml2ZXJzYWxBZGRyZXNzEkJCQGM2ZTIwM2M5OTY2YWRjNDEwZmM2M2UyMmFkOGU0N2E4YTMyYWIwMzExZWJjMThiNTQ2NzEwZGVmMTFhODVmM2MK0AEKzQFiygEKxwEKAhgEEsABar0BClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBiMWZmNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBjMmFhNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmCgQKAhgkCmEKX3JdChJUcmFuc2ZlckZhY3RvcnlDaWQSR0pFAChHBcKc87wzEsfeGAJkFwf/Gq0f41n0MHv4y4XCMdy9yhISIHh0D5Az9+lCpMLJRY9od0oWLUWcS4VbOXWuj1p/tiO5CgQKAhAACgQKAhgACgQKAhgCKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Kl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5xiNZdQ1hBgBCKgomCiQIARIgw9ZHAi5dLvhfUq0ujmSjGEL0vRvLhH31raAWp9V+gxsQHg==",
              interfaceViews: [],
              witnessParties: [
                "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [],
              createdAt: "2026-11-24T22:44:51.509190Z",
              packageName: "ntt",
              representativePackageId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIuwUSBI39ny4=",
      },
    ],
  },
  accept: {
    envelope: {
      commands: [
        {
          ExerciseCommand: {
            templateId:
              "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:AdminTransferProposal",
            contractId:
              "004070112634e557f65b7fe8a72103822ad67a97d1c97bda02f0d059e10b3fa799ca12122053ceea51ffdd5b5a47139a2276291be4d1886a7425ec8a8abd847f6df5e35a1d",
            choice: "AcceptAdminTransfer",
            choiceArgument: {
              mgrCid:
                "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
              ledgerCid:
                "004e01ad3acdd247f72346eeadc8dcc2192d7bda1df866c500191eba5e35b46287ca12122068cbb53478ee07eed2a10c30ff83e82f0b8710957b54f5d5fcdd0c75bccdd404",
              vettedFactory: {
                tag: "TransferFactoryCid",
                value:
                  "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
              },
              extraArgs: {
                context: {
                  values: {},
                },
                meta: {
                  values: {},
                },
              },
            },
          },
        },
      ],
      commandId:
        "wormhole-sdk-d295a7c1c48e8c96fe3ee88fe5254aec6ab8189d090e4a6aa50dbe3d290c929d",
      actAs: [
        "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
      ],
      disclosedContracts: [
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          createdEventBlob:
            "CgMyLjESsw0KRQCqrDVl69laudZHP2y37p6axVDu47VcyYhDhDqM0eCEncoSEiC/R/37GOTlyYRvT6RBCFRfQKzgKtjGmtFeCdxdTuI2HRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIinAlqmQkKSApGQkRudHQ6NTY5MTQ0ZmJhMDE2MzFkZWY4OTg3NzUzNWM0NjNlMTNkNTc5NTMzODg3Zjc0MjNmZGMzYzY3Y2RiZTc4ZTE4NwpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpjCmE6X1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgEClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjCgQKAhgECoQBCoEBan8KawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ChAKDkIMc2RrLW93bmVkLXM1ChAKDnoMCgpMb2NrVW5sb2NrCgQKAhgUCloKWHJWChBVbml2ZXJzYWxBZGRyZXNzEkJCQGM2ZTIwM2M5OTY2YWRjNDEwZmM2M2UyMmFkOGU0N2E4YTMyYWIwMzExZWJjMThiNTQ2NzEwZGVmMTFhODVmM2MK0AEKzQFiygEKxwEKAhgEEsABar0BClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBiMWZmNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBjMmFhNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmCgQKAhgkCmEKX3JdChJUcmFuc2ZlckZhY3RvcnlDaWQSR0pFAChHBcKc87wzEsfeGAJkFwf/Gq0f41n0MHv4y4XCMdy9yhISIHh0D5Az9+lCpMLJRY9od0oWLUWcS4VbOXWuj1p/tiO5CgQKAhAACgQKAhgACgQKAhgCKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Kl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5xiNZdQ1hBgBCKgomCiQIARIgw9ZHAi5dLvhfUq0ujmSjGEL0vRvLhH31raAWp9V+gxsQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
        {
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "004e01ad3acdd247f72346eeadc8dcc2192d7bda1df866c500191eba5e35b46287ca12122068cbb53478ee07eed2a10c30ff83e82f0b8710957b54f5d5fcdd0c75bccdd404",
          createdEventBlob:
            "CgMyLjES+wUKRQBOAa06zdJH9yNG7q3I3MIZLXvaHfhmxQAZHrpeNbRih8oSEiBoy7U0eO4H7tKhDDD/g+gvC4cQlXtU9dX83Qx1vM3UBBIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIsQCasECCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjChAKDjIMMC4wMDAwMDAwMDAwKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Ml9TZGtPd25lZEFkbWluLXM1LWZiOWEwZjU4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjkUpVJ1DWEGAEIqCiYKJAgBEiAOCjq7H68WY9Yt8fWcdt+KU6Qpq9kYSK0ljENWZeUTWxAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
        },
      ],
      synchronizerId:
        "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
      packageIdSelectionPreference: [
        "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
      ],
    },
    acs: [
      {
        workflowId: "",
        contractEntry: {
          JsActiveContract: {
            createdEvent: {
              offset: 533,
              nodeId: 0,
              contractId:
                "004070112634e557f65b7fe8a72103822ad67a97d1c97bda02f0d059e10b3fa799ca12122053ceea51ffdd5b5a47139a2276291be4d1886a7425ec8a8abd847f6df5e35a1d",
              templateId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:AdminTransferProposal",
              contractKey: null,
              contractKeyHash: "",
              createArgument: {
                admin:
                  "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                newAdmin:
                  "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
                managerAddress: {
                  tag: "NttManagerAddress",
                  value:
                    "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
                },
              },
              createdEventBlob:
                "CgMyLjES2QUKRQBAcBEmNOVX9lt/6KchA4Iq1nqX0cl72gLw0FnhCz+nmcoSEiBTzupR/91bWkcTmiJ2KRvk0YhqdCXsioq9hH9t9eNaHRIDbnR0GnEKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaFUFkbWluVHJhbnNmZXJQcm9wb3NhbCKlAmqiAgpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Cl4KXDpaU2RrT3RoZXItczUtMmQwY2QwZjE6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjKl9TZGtPd25lZEFkbWluLXM1LWZiOWEwZjU4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjJaU2RrT3RoZXItczUtMmQwY2QwZjE6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OTLuN3oNYQYAQioKJgokCAESIHe+Do3WaD9MzGAArrngK/ImuvLU8kFU2O0S5s83sHpkEB4=",
              interfaceViews: [],
              witnessParties: [
                "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              signatories: [
                "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              observers: [
                "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              ],
              createdAt: "2026-11-24T22:46:13.218866Z",
              packageName: "ntt",
              representativePackageId:
                "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d",
              acsDelta: true,
            },
            synchronizerId:
              "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
            reassignmentCounter: 0,
          },
        },
        streamContinuationToken: "CgMIvgYSBA+zFNY=",
      },
    ],
    flowUrl:
      "/v1/flows/transfer?manager=9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
    flow: {
      flow: "transfer",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          createdEventBlob:
            "CgMyLjESsw0KRQCqrDVl69laudZHP2y37p6axVDu47VcyYhDhDqM0eCEncoSEiC/R/37GOTlyYRvT6RBCFRfQKzgKtjGmtFeCdxdTuI2HRIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIinAlqmQkKSApGQkRudHQ6NTY5MTQ0ZmJhMDE2MzFkZWY4OTg3NzUzNWM0NjNlMTNkNTc5NTMzODg3Zjc0MjNmZGMzYzY3Y2RiZTc4ZTE4NwpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpjCmE6X1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmMKYTpfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgEClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjCgQKAhgECoQBCoEBan8KawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ChAKDkIMc2RrLW93bmVkLXM1ChAKDnoMCgpMb2NrVW5sb2NrCgQKAhgUCloKWHJWChBVbml2ZXJzYWxBZGRyZXNzEkJCQGM2ZTIwM2M5OTY2YWRjNDEwZmM2M2UyMmFkOGU0N2E4YTMyYWIwMzExZWJjMThiNTQ2NzEwZGVmMTFhODVmM2MK0AEKzQFiygEKxwEKAhgEEsABar0BClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBiMWZmNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDBjMmFhNWU4YjJjMWYzYTZkMGE5YjFjMmQzZTRmNWE2YjdjOGQ5ZTBmCgQKAhgkCmEKX3JdChJUcmFuc2ZlckZhY3RvcnlDaWQSR0pFAChHBcKc87wzEsfeGAJkFwf/Gq0f41n0MHv4y4XCMdy9yhISIHh0D5Az9+lCpMLJRY9od0oWLUWcS4VbOXWuj1p/tiO5CgQKAhAACgQKAhgACgQKAhgCKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Kl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNipfU2RrT3duZWRBZG1pbi1zNS1mYjlhMGY1ODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5xiNZdQ1hBgBCKgomCiQIARIgw9ZHAi5dLvhfUq0ujmSjGEL0vRvLhH31raAWp9V+gxsQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:569144fba01631def89877535c463e13d579533887f7423fdc3c67cdbe78e187",
            admin:
              "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "2",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
            },
            transceiverEmitterId: "2",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-owned-s5",
            },
            tokenConfig: "LockUnlock",
            tokenDecimals: "10",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "c6e203c9966adc410fc63e22ad8e47a8a32ab0311ebc18b546710def11a85f3c",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "TransferFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "transceiver Emitter",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "00cdd0efa373db9e0fad922cfd78998376d636415ef0715ffcf7b072037db48f9fca12122094f240a1bfe2f255e697fe7792a8195f9c87f80148d8c2efdb4281f4b665f1c2",
          createdEventBlob:
            "CgMyLjESngUKRQDN0O+jc9ueD62SLP14mYN21jZBXvBxX/z3sHIDfbSPn8oSEiCU8kChv+LyVeaX/neSqBlfnIf4AUjYwu/bQoH0tmXxwhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYBApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJANTQ3YzFhYTQ4ZDBkN2UxOGRmMDZiYWY4YzY0MWRkZTdiZjg0MzdhZjg5MWRhNjcxMTI4NDVmZTkxOWUzMWY0OQoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnGI1l1DWEGAEIqCiYKJAgBEiDBQ5iTdSKFeto6VMQLsXjrHuEX1XJ2lXWtN0PY4RaTzhAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            owner:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            emitterId: "2",
            emitterAddress: {
              tag: "EmitterAddress",
              value:
                "547c1aa48d0d7e18df06baf8c641dde7bf8437af891da67112845fe919e31f49",
            },
            sequence: "2",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "LockedLedger",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "004e01ad3acdd247f72346eeadc8dcc2192d7bda1df866c500191eba5e35b46287ca12122068cbb53478ee07eed2a10c30ff83e82f0b8710957b54f5d5fcdd0c75bccdd404",
          createdEventBlob:
            "CgMyLjES+wUKRQBOAa06zdJH9yNG7q3I3MIZLXvaHfhmxQAZHrpeNbRih8oSEiBoy7U0eO4H7tKhDDD/g+gvC4cQlXtU9dX83Qx1vM3UBBIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIsQCasECCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpjCmE6X1Nka093bmVkQWRtaW4tczUtZmI5YTBmNTg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2ClsKWXJXChFOdHRNYW5hZ2VyQWRkcmVzcxJCQkA5YjkxN2I0MjBjYzE2MTkwYmIxYWUxYzYyOGRlMzIzOTg1M2UzYmZiMDg5MGI1MDk2MzA3OTE1NTUyYmYxNGJjChAKDjIMMC4wMDAwMDAwMDAwKmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2Ml9TZGtPd25lZEFkbWluLXM1LWZiOWEwZjU4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjkUpVJ1DWEGAEIqCiYKJAgBEiAOCjq7H68WY9Yt8fWcdt+KU6Qpq9kYSK0ljENWZeUTWxAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            admin:
              "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
            },
            balance: "0.0000000000",
          },
        },
        {
          role: "committed TransferFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
      ],
      missing: [
        {
          role: "custodian TransferPreapproval",
          reason: "no matching contract visible to the reader parties",
        },
      ],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "005ee80c79595492aa848f99bd980978b07f0cdce88990db20504687ccf42fb8c8ca12122019b54f4a5a4b995c3b59621a56aa7a7c0c6e4afd433ba07c173c85a8775ee027",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
            "21": {
              managerAddress:
                "5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c",
              transceiverAddress:
                "5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d",
              decimals: 8,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00aaac3565ebd95ab9d6473f6cb7ee9e9ac550eee3b55cc98843843a8cd1e0849dca121220bf47fdfb18e4e5c9846f4fa44108545f40ace02ad8c69ad15e09dc5d4ee2361d",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOwnedAdmin-s5-fb9a0f58::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
  },
  acceptBack: {
    flowUrl:
      "/v1/flows/transfer?manager=9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
    flow: {
      flow: "transfer",
      disclosures: [
        {
          role: "NttManager",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Manager:NttManager",
          contractId:
            "00ad520640cc4470493075d8d97b84807ba10630c38f051364b001d31d402321dcca12122051cdcbd2d8ffb1ab36881877ddcfc28e1077f15ac321d07b6260e18d1e15118a",
          createdEventBlob:
            "CgMyLjESpA0KRQCtUgZAzERwSTB12Nl7hIB7oQYww48FE2SwAdMdQCMh3MoSEiBRzcvS2P+xqzaIGHfdz8KOEHfxWsMh0HtiYOGNHhURihIDbnR0GmYKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSB01hbmFnZXIaCk50dE1hbmFnZXIikglqjwkKSApGQkRudHQ6NTY5MTQ0ZmJhMDE2MzFkZWY4OTg3NzUzNWM0NjNlMTNkNTc5NTMzODg3Zjc0MjNmZGMzYzY3Y2RiZTc4ZTE4NwpeClw6WlNka090aGVyLXM1LTJkMGNkMGYxOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKYwphOl9TZGtOdHRDdXN0b2R5LXM1LWM4MTJhNzZkOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpeClw6WlNka090aGVyLXM1LTJkMGNkMGYxOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoFCgMYlgEKBAoCGAQKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDliOTE3YjQyMGNjMTYxOTBiYjFhZTFjNjI4ZGUzMjM5ODUzZTNiZmIwODkwYjUwOTYzMDc5MTU1NTJiZjE0YmMKBAoCGAQKhAEKgQFqfwprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKEAoOQgxzZGstb3duZWQtczUKEAoOegwKCkxvY2tVbmxvY2sKBAoCGBQKWgpYclYKEFVuaXZlcnNhbEFkZHJlc3MSQkJAYzZlMjAzYzk5NjZhZGM0MTBmYzYzZTIyYWQ4ZTQ3YThhMzJhYjAzMTFlYmMxOGI1NDY3MTBkZWYxMWE4NWYzYwrQAQrNAWLKAQrHAQoCGAQSwAFqvQEKWwpZclcKEU50dE1hbmFnZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGIxZmY1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKWApWclQKDkVtaXR0ZXJBZGRyZXNzEkJCQDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMGMyYWE1ZThiMmMxZjNhNmQwYTliMWMyZDNlNGY1YTZiN2M4ZDllMGYKBAoCGCQKYQpfcl0KElRyYW5zZmVyRmFjdG9yeUNpZBJHSkUAKEcFwpzzvDMSx94YAmQXB/8arR/jWfQwe/jLhcIx3L3KEhIgeHQPkDP36UKkwslFj2h3ShYtRZxLhVs5da6PWn+2I7kKBAoCEAAKBAoCGAAKBAoCGAIqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqX1Nka050dEN1c3RvZHktczUtYzgxMmE3NmQ6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2KlpTZGtPdGhlci1zNS0yZDBjZDBmMTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY5ojQ9eg1hBgBCKgomCiQIARIgVkVJfqNavYpvIkQjKdbtSMdMIyD8BoxvkRaMcdKnZR0QHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            namespace:
              "ntt:569144fba01631def89877535c463e13d579533887f7423fdc3c67cdbe78e187",
            admin:
              "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            nttCustody:
              "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            reserveCustodian:
              "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            managerId: "2",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
            },
            transceiverEmitterId: "2",
            instrumentId: {
              admin:
                "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "sdk-owned-s5",
            },
            tokenConfig: "LockUnlock",
            tokenDecimals: "10",
            tokenAddress: {
              tag: "UniversalAddress",
              value:
                "c6e203c9966adc410fc63e22ad8e47a8a32ab0311ebc18b546710def11a85f3c",
            },
            peers: [
              [
                "2",
                {
                  managerAddress: {
                    tag: "NttManagerAddress",
                    value:
                      "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  transceiverAddress: {
                    tag: "EmitterAddress",
                    value:
                      "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
                  },
                  decimals: "18",
                },
              ],
            ],
            factory: {
              tag: "TransferFactoryCid",
              value:
                "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
            },
            paused: false,
            factoryEpoch: "0",
            peerEpoch: "1",
          },
        },
        {
          role: "transceiver Emitter",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:Emitter",
          contractId:
            "00cdd0efa373db9e0fad922cfd78998376d636415ef0715ffcf7b072037db48f9fca12122094f240a1bfe2f255e697fe7792a8195f9c87f80148d8c2efdb4281f4b665f1c2",
          createdEventBlob:
            "CgMyLjESngUKRQDN0O+jc9ueD62SLP14mYN21jZBXvBxX/z3sHIDfbSPn8oSEiCU8kChv+LyVeaX/neSqBlfnIf4AUjYwu/bQoH0tmXxwhINd29ybWhvbGUtY29yZRpiCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaB0VtaXR0ZXIiwwJqwAIKawppOmdTZGtHdWFyZGlhbkdvdmVybmFuY2UtczUtZDRkOTUxMzg6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2CmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgIYBApYClZyVAoORW1pdHRlckFkZHJlc3MSQkJANTQ3YzFhYTQ4ZDBkN2UxOGRmMDZiYWY4YzY0MWRkZTdiZjg0MzdhZjg5MWRhNjcxMTI4NDVmZTkxOWUzMWY0OQoECgIYBCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjnGI1l1DWEGAEIqCiYKJAgBEiDBQ5iTdSKFeto6VMQLsXjrHuEX1XJ2lXWtN0PY4RaTzhAe",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            owner:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            emitterId: "2",
            emitterAddress: {
              tag: "EmitterAddress",
              value:
                "547c1aa48d0d7e18df06baf8c641dde7bf8437af891da67112845fe919e31f49",
            },
            sequence: "2",
          },
        },
        {
          role: "CoreState",
          templateId:
            "32abbcd2a6b5ebc2530f8e2cfc3fd1b853ed496443c6a0125fd2cea617b33371:Wormhole.Core.State:CoreState",
          contractId:
            "001f8894b67f7ae64b3b2c1733b8d15a3d75bdd391f959cbea64c7140508e00bd8ca121220bb61d85bb9429b62b5f87061cd678535696ef9d42ee701cf5fe4a98ad2e558e0",
          createdEventBlob:
            "CgMyLjESnQkKRQAfiJS2f3rmSzssFzO40Vo9db3TkflZy+pkxxQFCOAL2MoSEiC7YdhbuUKbYrX4cGHNZ4U1aW751C7nAc9f5KmK0uVY4BINd29ybWhvbGUtY29yZRpkCkAzMmFiYmNkMmE2YjVlYmMyNTMwZjhlMmNmYzNmZDFiODUzZWQ0OTY0NDNjNmEwMTI1ZmQyY2VhNjE3YjMzMzcxEghXb3JtaG9sZRIEQ29yZRIFU3RhdGUaCUNvcmVTdGF0ZSLfBWrcBQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKBQoDGJYBCgQKAhgCClgKVnJUCg5FbWl0dGVyQWRkcmVzcxJCQkAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA0CgQKAhgACo8CCowCYokCCoYCCgIYABL/AWr8AQr5AQr2AVrzAQrwAWrtAQpBCj9yPQoPR3VhcmRpYW5BZGRyZXNzEipCKGJlZmE0MjlkNTdjZDE4YjdmOGE0ZDkxYTJkYTlhYjRhZjA1ZDBmYmUKpwEKpAFSoQEKngFymwEKEUd1YXJkaWFuUHVibGljS2V5EoUBQoIBMDRkNGE0NjI5OTc5ZjBjOWZhMGYwYmI1NGVkZjMzZjg3YzhjNWExZjQyYzAzNTBhMzBkNjhmN2U5NjcwMjNlMzRlNDk1YThlYmY1MTAxMDM2ZDBmZDY2ZTNiMGE4YzdjNjFiNjVmY2VlYWY0ODdhYjNjZDFiNWI3YjUwYmViNzk3MAoECgIYAAp9CntqeQprCmk6Z1Nka0ZlZUluc3RydW1lbnRBZG1pbi1zNS00ZjFkZjAzYTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYKCgoIQgZVTlVTRUQKYwphOl9TZGtGZWVDdXN0b2R5LXM1LTliMzk3MGJlOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgoECgJiACpfU2RrRmVlQ3VzdG9keS1zNS05YjM5NzBiZTo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY50fYPdQ1hBgBCKgomCiQIARIgBmrmpy76t5mLdK7C67IWp95zGyMpiQ8aL/hF5AYH2YEQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            chainId: "75",
            governanceChainId: "1",
            governanceContract: {
              tag: "EmitterAddress",
              value:
                "0000000000000000000000000000000000000000000000000000000000000004",
            },
            guardianSetIndex: "0",
            guardianSets: [
              [
                "0",
                {
                  members: [
                    {
                      address: {
                        tag: "GuardianAddress",
                        value: "befa429d57cd18b7f8a4d91a2da9ab4af05d0fbe",
                      },
                      publicKey: {
                        tag: "GuardianPublicKey",
                        value:
                          "04d4a4629979f0c9fa0f0bb54edf33f87c8c5a1f42c0350a30d68f7e967023e34e495a8ebf5101036d0fd66e3b0a8c7c61b65fceeaf487ab3cd1b5b7b50beb7970",
                      },
                    },
                  ],
                },
              ],
            ],
            messageFee: "0",
            feeInstrument: {
              admin:
                "SdkFeeInstrumentAdmin-s5-4f1df03a::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
              id: "UNUSED",
            },
            feeRecipient:
              "SdkFeeCustody-s5-9b3970be::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            consumedGovernance: [],
          },
        },
        {
          role: "LockedLedger",
          templateId:
            "43e1b2060c420d9bbd78ebeaeca1c30b58ee967fd40b5aacd58088d6f239a63d:Wormhole.Ntt.Ledger:LockedLedger",
          contractId:
            "00752d9bedee59a172b0edbbab6ed1cfbaffb891ed839458815687cae300bd77cfca12122063cdf9240803ea383094807eb7eb21dfa3741bf558079d3cece76c195b765fe1",
          createdEventBlob:
            "CgMyLjES8QUKRQB1LZvt7lmhcrDtu6tu0c+6/7iR7YOUWIFWh8rjAL13z8oSEiBjzfkkCAPqODCUgH636yHfo3Qb9VgHnTzs52wZW3Zf4RIDbnR0GmcKQDQzZTFiMjA2MGM0MjBkOWJiZDc4ZWJlYWVjYTFjMzBiNThlZTk2N2ZkNDBiNWFhY2Q1ODA4OGQ2ZjIzOWE2M2QSCFdvcm1ob2xlEgNOdHQSBkxlZGdlchoMTG9ja2VkTGVkZ2VyIr8CarwCCmsKaTpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpeClw6WlNka090aGVyLXM1LTJkMGNkMGYxOjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNgpbCllyVwoRTnR0TWFuYWdlckFkZHJlc3MSQkJAOWI5MTdiNDIwY2MxNjE5MGJiMWFlMWM2MjhkZTMyMzk4NTNlM2JmYjA4OTBiNTA5NjMwNzkxNTU1MmJmMTRiYwoQCg4yDDAuMDAwMDAwMDAwMCpnU2RrR3VhcmRpYW5Hb3Zlcm5hbmNlLXM1LWQ0ZDk1MTM4OjoxMjIwMDBmMjRkYjRjNWJmZmY1MDhkNDcyOWY0ZGFhZjk4YWU5MzViZDU3YzgyMmY1ZjVjMWQwNTllZmQ1YTU4YjZjNjJaU2RrT3RoZXItczUtMmQwY2QwZjE6OjEyMjAwMGYyNGRiNGM1YmZmZjUwOGQ0NzI5ZjRkYWFmOThhZTkzNWJkNTdjODIyZjVmNWMxZDA1OWVmZDVhNThiNmM2OaI0PXoNYQYAQioKJgokCAESIJVe9ILgS8JIOCBFQ4RnVm5Qna3m5QjlyQSOhhzoZQ4IEB4=",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            guardianGovernance:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            admin:
              "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            managerAddress: {
              tag: "NttManagerAddress",
              value:
                "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
            },
            balance: "0.0000000000",
          },
        },
        {
          role: "committed TransferFactory",
          templateId:
            "0a624d6a07264860159e881ca7ff6b349f58e95f393c8861529089f9cd8ba474:Token.CIP0056.CoinFactory:CoinFactory",
          contractId:
            "00284705c29cf3bc3312c7de1802641707ff1aad1fe359f4307bf8cb85c231dcbdca12122078740f9033f7e942a4c2c9458f68774a162d459c4b855b3975ae8f5a7fb623b9",
          createdEventBlob:
            "CgMyLjES0wMKRQAoRwXCnPO8MxLH3hgCZBcH/xqtH+NZ9DB7+MuFwjHcvcoSEiB4dA+QM/fpQqTCyUWPaHdKFi1FnEuFWzl1ro9af7YjuRINdG9rZW4tY2lwMDA1NhpsCkAwYTYyNGQ2YTA3MjY0ODYwMTU5ZTg4MWNhN2ZmNmIzNDlmNThlOTVmMzkzYzg4NjE1MjkwODlmOWNkOGJhNDc0EgVUb2tlbhIHQ0lQMDA1NhILQ29pbkZhY3RvcnkaC0NvaW5GYWN0b3J5Im9qbQprCmk6Z1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzYqZ1Nka0d1YXJkaWFuR292ZXJuYW5jZS1zNS1kNGQ5NTEzODo6MTIyMDAwZjI0ZGI0YzViZmZmNTA4ZDQ3MjlmNGRhYWY5OGFlOTM1YmQ1N2M4MjJmNWY1YzFkMDU5ZWZkNWE1OGI2YzY51ZkrdQ1hBgBCKgomCiQIARIg1Tp0pPP1wVcMVpP09vfL8bTK1frmgFxPKmGjJf15BlMQHg==",
          synchronizerId:
            "synchronizer-1::12201fd1b2a0bb1b2dcafe05188881de2bf18f0087ac635ef89563f0ec8bc8d787a3",
          createArgument: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          },
        },
      ],
      missing: [
        {
          role: "custodian TransferPreapproval",
          reason: "no matching contract visible to the reader parties",
        },
      ],
    },
    managers: {
      managers: [
        {
          managerAddress:
            "00469217ffb1fb8a5a6d874618c690c12222848a67ed2820e2f58b5a3c513382",
          contractId:
            "005ee80c79595492aa848f99bd980978b07f0cdce88990db20504687ccf42fb8c8ca12122019b54f4a5a4b995c3b59621a56aa7a7c0c6e4afd433ba07c173c85a8775ee027",
          chainId: 75,
          managerId: "0",
          tokenConfig: "BurnMint",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "wormhole-ntt:7b6e0583496475490190fef927d42b173880d12813de8d07218c4091e15655a8",
          },
          reserveCustodian:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 8,
          admin:
            "SdkBurnAdmin-s5-40b5f04b::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
            "21": {
              managerAddress:
                "5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c",
              transceiverAddress:
                "5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d5d",
              decimals: 8,
            },
          },
        },
        {
          managerAddress:
            "26f50a5442bda04fdde78782ebb85daccea4d7265608ed16f22d17c29d6ea6f2",
          contractId:
            "0066e6144fe849c1ab319858a94f02194b9b465ea283a7f69696c76d1cd2c7a015ca12122012394c45495a0b68a3ba957625ac91e62a987b90b90c250e16f8cb9951af78c9",
          chainId: 75,
          managerId: "1",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-lock-s5",
          },
          reserveCustodian:
            "SdkNttCustody-s5-c812a76d::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
        {
          managerAddress:
            "9b917b420cc16190bb1ae1c628de3239853e3bfb0890b5096307915552bf14bc",
          contractId:
            "00ad520640cc4470493075d8d97b84807ba10630c38f051364b001d31d402321dcca12122051cdcbd2d8ffb1ab36881877ddcfc28e1077f15ac321d07b6260e18d1e15118a",
          chainId: 75,
          managerId: "2",
          tokenConfig: "LockUnlock",
          instrumentId: {
            admin:
              "SdkGuardianGovernance-s5-d4d95138::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
            id: "sdk-owned-s5",
          },
          reserveCustodian:
            "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          paused: false,
          tokenDecimals: 10,
          admin:
            "SdkOther-s5-2d0cd0f1::122000f24db4c5bfff508d4729f4daaf98ae935bd57c822f5f5c1d059efd5a58b6c6",
          peers: {
            "2": {
              managerAddress:
                "000000000000000000000000b1ff5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              transceiverAddress:
                "000000000000000000000000c2aa5e8b2c1f3a6d0a9b1c2d3e4f5a6b7c8d9e0f",
              decimals: 18,
            },
          },
        },
      ],
    },
  },
};
