# Notes on changes made in this integration branch

This file is meant to document the work in progress on this integration branch. It can probably be deleted (or the information moved some place better) before this PR is merged.

This integration branch is PR #21.

## Sub Branches

Note that the indentation indicates branch dependencies.

- **evm_TransceiverRegistry_split** (PR #22) - This branch splits transceiver admin into a separate contract.
  - **evm_per_chain_transceivers** (PR #26) - This branch adds per-chain transceiver and per-chain threshold support.
- **evm/add_MsgManager** (PR #23) - Creates `MsgManagerBase` and `MsgManager` and makes `NttManager` inherit from `MsgManagerBase`.
  - **evm/add_SharedWormholeTransceiver** (PR #25) - Adds a shareable transceiver.

## The change that wasn't made

It is unfortunate that `token` and `mode` exist in `ManagerBase` rather than `NttManager`.
I tried moving them, but that increases the size of `NttManagerNoRateLimiting` considerably.
I'm not sure why that is, or how to avoid it, so I did not pursue that change at this time.
However, some of the other changes also cause that increase, so maybe we can revist this.

## Contract size before we started

```bash
evm (main)$ forge build --sizes --via-ir --skip test
╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,066           | 25,673            | 510                | 23,479              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 17,141           | 18,557            | 7,435              | 30,595              |
╰-----------------------------------------+------------------+-------------------+--------------------+---------------------╯
```

## Contract size after moving token and mode (the change that wasn't made)

```bash
evm (main)$ forge build --sizes --via-ir --skip test
╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,066           | 25,676            | 510                | 23,476              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 18,788           | 20,281            | 5,788              | 28,871              |
╰-----------------------------------------+------------------+-------------------+--------------------+---------------------╯
```

## Creating TransceiverRegistryAdmin (PR #22)

This change creates a separate contract to perform transceiver administration and updates the standard `TransceiverRegistry`
to delegate call into the new contract. This reduces the overall size of `TransceiverRegistry` and therefore `NttManager`.

Currently in this PR, `TransceiverRegistry` instantiates `TransceiverRegistryAdmin` in the constructor, so that I didn't have to update
all of the deployment code. I think it should actually be passed in as a constructor parameter and be updatable. (I'm not
saying that `TransceiverRegistryAdmin` needs to be upgradeable. We could just make the attribute in `TransceiverRegistry`
mutable.)

### Possible Enhancement

We could get rid of `TransceiverRegistry` altogether, and move the functionality into `ManagerBase` and change `TransceiverRegistryAdmin`
to `ManagerBaseAdmin`. That would allow us to also move the admin code in `ManagerBase`into there, and further reduce the
size of`NttManager`.

### Contract Size with TransceiverRegistryAdmin

```bash
evm (main)$ forge build --sizes --via-ir --skip test
╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 23,220           | 26,937            | 1,356              | 22,215              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 16,254           | 19,713            | 8,322              | 29,439              |
╰-----------------------------------------+------------------+-------------------+--------------------+---------------------╯
```

## Adding Per-Chain Transceivers (on top of TransceiverRegistryAdmin) (PR #26)

This change allows you to specify different sets of transceivers for each destination chain, as well as different sets
for sending and receiving. Additionally, it allows you to specify a different threshold for each destination chain.

Note that there is no default set of send / receive transceivers. They must be enabled for each chain. This must be handled
during contract migration.

### Contract Size with Per-Chain Transceivers

```bash
evm (main)$ forge build --sizes --via-ir --skip test
╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,089           | 31,480            | 487                | 17,672              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 17,150           | 24,275            | 7,426              | 24,877              |
╰-----------------------------------------+------------------+-------------------+--------------------+---------------------╯
```

## Creating MsgManagerBase (PR #23)

This change adds support for generic message passing. It also updates `NttManager`
to use it.

### Contract Size with MsgManagerBase

```bash
evm (main)$ forge build --sizes --via-ir --skip test
╭-----------------------------------------+------------------+-------------------+--------------------+---------------------╮
| Contract                                | Runtime Size (B) | Initcode Size (B) | Runtime Margin (B) | Initcode Margin (B) |
+===========================================================================================================================+
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManager                              | 24,076           | 25,719            | 500                | 23,433              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| NttManagerNoRateLimiting                | 18,496           | 19,949            | 6,080              | 29,203              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| MsgManager                              | 12,540           | 13,745            | 12,036             | 35,407              |
|-----------------------------------------+------------------+-------------------+--------------------+---------------------|
| MsgManagerWithExecutor                  | 13,145           | 14,400            | 11,431             | 34,752              |
╰-----------------------------------------+------------------+-------------------+--------------------+---------------------╯
```

## Shared Transceiver (PR #25)

This change allows a Wormhole transceiver to be shared between multiple `NttManagers`. This transceiver does not support relaying
because it is assumed that the integrator will call the `Executor` at a higher level, using something like `NttManagerWithExecutor`.
