# Notes on changes made in this integration branch

This file is meant to document the work in progress on this integration branch. It can probably be deleted (or the information moved some place better) before this PR is merged.

This integration branch is PR #21.

## Sub Branches

Note that the indentation indicates branch dependencies.

- **evm_TransceiverRegistry_split** (PR #22) - This branch splits transceiver admin into a separate contract.
  - **evm_per_chain_transceivers** (PR #26) - This branch adds per-chain transceiver support, should also add per-chain thresholds.
- **evm/add_MsgManager** (PR #23) - Creates `MsgManagerBase` and `MsgManager` and makes `NttManager` inherit from `MsgManagerBase`.
  - **evm/add_SharedWormholeTransceiver** (PR #25) - Adds a shareable transceiver.

## The change that wasn't made

It is unfortunate that `token` and `mode` exist in `ManagerBase` rather than `NttManager`.
I tried moving them, but that increases the size of `NttManagerNoRateLimiting` considerably.
I'm not sure why that is, or how to avoid it, so I did not pursue that change at this time.
However, some of the other changes also cause that increase, so maybe we can revist this.

## Contract Sizes

### Before we started

```bash
evm (main)$ forge build --sizes --via-ir --skip test

╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,066           | 25,673            | 510                | 23,479              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 17,141           | 18,557            | 7,435              | 30,595              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|

```

### After moving token and mode (the change that wasn't made)

```bash
evm (main)$ forge build --sizes --via-ir --skip test

|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,066           | 25,676            | 510                | 23,476              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 18,788           | 20,281            | 5,788              | 28,871              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|

```

## Creating TransceiverRegistryAdmin

```bash
evm (main)$ forge build --sizes --via-ir --skip test

|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 23,220           | 26,937            | 1,356              | 22,215              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 16,254           | 19,713            | 8,322              | 29,439              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|

```

## Creating MsgManagerBase

```bash
evm (main)$ forge build --sizes --via-ir --skip test

|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,076           | 25,719            | 500                | 23,433              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 18,496           | 19,949            | 6,080              | 29,203              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| MsgManager                              | 12,540           | 13,745            | 12,036             | 35,407              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| MsgManagerWithExecutor                  | 13,145           | 14,400            | 11,431             | 34,752              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|

```
