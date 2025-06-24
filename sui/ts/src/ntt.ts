import {
  AccountAddress,
  ChainAddress,
  UnsignedTransaction,
} from "@wormhole-foundation/sdk-definitions";
import type { Chain, Network } from "@wormhole-foundation/sdk-base";
import {
  Ntt,
  NttTransceiver,
} from "@wormhole-foundation/sdk-definitions-ntt";
import { SuiChains, SuiUnsignedTransaction } from "@wormhole-foundation/sdk-sui";
import { SuiClient } from "@mysten/sui/client";
import { Transaction } from "@mysten/sui/transactions";

export class SuiNtt<N extends Network, C extends SuiChains> implements Ntt<N, C> {
  readonly network: N;
  readonly chain: C;
  readonly provider: SuiClient;
  readonly contracts: Ntt.Contracts;
  readonly adminCapId?: string; // NTT AdminCap object ID
  readonly packageId?: string; // NTT package ID for move calls (TODO: do we need this? or just infer from the admincap)

  constructor(
    network: N,
    chain: C,
    provider: SuiClient,
    contracts: any, // TODO: Fix type - should be platform contracts + ntt
    adminCapId?: string,
    packageId?: string
  ) {
    if (!contracts.ntt) {
      throw new Error("NTT contracts not found");
    }

    this.network = network;
    this.chain = chain;
    this.provider = provider;
    this.contracts = contracts.ntt;
    this.adminCapId = adminCapId;
    this.packageId = packageId;
  }

  static async fromRpc<N extends Network>(
    provider: SuiClient,
    config: any
  ): Promise<SuiNtt<N, SuiChains>> {
    // Determine network and chain from RPC
    const network = "Mainnet" as N; // TODO: Detect network from RPC
    const chain = "Sui" as SuiChains;

    // Extract NTT configuration from the config
    const nttConfig = config.Sui?.contracts?.ntt;
    if (!nttConfig) {
      throw new Error("NTT configuration not found");
    }

    // Create contracts configuration
    const contracts: Ntt.Contracts = {
      token: nttConfig.token || "",
      manager: nttConfig.manager,
      transceiver: nttConfig.transceiver || {},
    };

    // Extract AdminCap ID and package ID if available
    const adminCapId = config.Sui?.adminCaps?.ntt;
    const packageId = config.Sui?.packageIds?.ntt;

    return new SuiNtt(network, chain, provider, { ntt: contracts }, adminCapId, packageId);
  }

  // State & Configuration Methods
  async getMode(): Promise<Ntt.Mode> {
    const state = await this.provider.getObject({
      id: this.contracts.manager,
      options: {
        showContent: true,
      },
    });

    if (!state.data?.content || state.data.content.dataType !== "moveObject") {
      throw new Error("Failed to fetch NTT state object");
    }

    const fields = (state.data.content as any).fields;
    const modeField = fields.mode;


    // Mode is an enum with a variant field: { variant: "Locking" } or { variant: "Burning" }
    if (modeField.variant === "Locking") {
      return "locking";
    } else if (modeField.variant === "Burning") {
      return "burning";
    }

    throw new Error("Invalid mode in NTT state");
  }

  async isPaused(): Promise<boolean> {
    // In Sui NTT, pausing is handled by the admin cap ownership
    // For now, return false as a placeholder
    // TODO: Implement proper pause checking mechanism
    return false;
  }

  async getOwner(): Promise<AccountAddress<C>> {
    // Owner is determined by who holds the AdminCap
    if (!this.adminCapId) {
      throw new Error("AdminCap ID not provided - cannot determine owner");
    }

    try {
      const adminCap = await this.provider.getObject({
        id: this.adminCapId,
        options: {
          showOwner: true,
        },
      });

      if (!adminCap.data?.owner) {
        throw new Error("Could not fetch AdminCap owner information");
      }

      // Extract owner address from the owner field
      let ownerAddress: string;
      if (typeof adminCap.data.owner === "object" && "AddressOwner" in adminCap.data.owner) {
        ownerAddress = adminCap.data.owner.AddressOwner;
      } else if (typeof adminCap.data.owner === "string") {
        ownerAddress = adminCap.data.owner;
      } else {
        throw new Error(`AdminCap has unexpected owner type: ${JSON.stringify(adminCap.data.owner)}`);
      }

      return ownerAddress as unknown as AccountAddress<C>;
    } catch (error) {
      throw new Error(`Failed to get AdminCap owner: ${error}`);
    }
  }

  async getPauser(): Promise<AccountAddress<C> | null> {
    // Pauser functionality would be similar to owner
    // For deployment purposes, return null (no pauser)
    return null;
  }

  async getThreshold(): Promise<number> {
    const state = await this.provider.getObject({
      id: this.contracts.manager,
      options: {
        showContent: true,
      },
    });

    if (!state.data?.content || state.data.content.dataType !== "moveObject") {
      throw new Error("Failed to fetch NTT state object");
    }

    const fields = (state.data.content as any).fields;
    return parseInt(fields.threshold, 10);
  }

  async getTokenDecimals(): Promise<number> {

    // For SUI token, decimals are always 9
    if (this.contracts.token === "0x2::sui::SUI") {
      return 9;
    }

    // For other tokens, we'd need to fetch from coin metadata
    // This requires knowing the token type parameter and querying the CoinMetadata object
    throw new Error(`getTokenDecimals not yet implemented for token: ${this.contracts.token}`);
  }

  async getCustodyAddress(): Promise<string> {
    // In Sui, custody is managed by the State object itself
    // Return the state object ID as the custody address
    return this.contracts.manager;
  }

  // Admin Methods
  async *pause(): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  async *unpause(): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  async *setOwner(newOwner: AccountAddress<C>, payer?: AccountAddress<C>): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  async *setPauser(newPauser: AccountAddress<C>, payer?: AccountAddress<C>): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  // Peer Management
  async *setPeer(
    peer: ChainAddress,
    tokenDecimals: number,
    inboundLimit: bigint
  ): AsyncGenerator<UnsignedTransaction<N, C>> {

    if (!this.adminCapId) {
      throw new Error("AdminCap ID required for setPeer operation");
    }

    if (!this.packageId) {
      throw new Error("Package ID required for setPeer operation");
    }

    // Import chainToChainId from SDK
    const { chainToChainId } = await import("@wormhole-foundation/sdk-base");

    // Build transaction to set peer
    const txb = new Transaction();

    // Convert chain to wormhole chain ID
    const wormholeChainId = chainToChainId(peer.chain);

    // Convert peer address to ExternalAddress format
    let peerAddressBytes: Uint8Array;
    try {
      if (!peer.address) {
        throw new Error("peer.address is null or undefined");
      }

      // Check if peer.address is already a UniversalAddress
      if (typeof peer.address.toUint8Array === 'function') {
        peerAddressBytes = peer.address.toUint8Array();
      } else if (typeof peer.address.toUniversalAddress === 'function') {
        const universalAddr = peer.address.toUniversalAddress();
        if (!universalAddr) {
          throw new Error("toUniversalAddress() returned null or undefined");
        }
        peerAddressBytes = universalAddr.toUint8Array();
      } else {
        throw new Error(`peer.address does not have expected methods. Type: ${typeof peer.address}, value: ${peer.address}, constructor: ${peer.address?.constructor?.name}`);
      }

      if (!peerAddressBytes || !(peerAddressBytes instanceof Uint8Array)) {
        throw new Error(`Address conversion failed: ${typeof peerAddressBytes}, expected Uint8Array`);
      }

    } catch (error) {
      throw new Error(`Failed to convert peer address to bytes: ${error instanceof Error ? error.message : String(error)}`);
    }

    // Convert Uint8Array to regular array before passing to txb.pure.vector
    let peerAddressBytesArray: number[];
    try {
      if (typeof peerAddressBytes.length === 'undefined') {
        throw new Error("peerAddressBytes.length is undefined");
      }

      peerAddressBytesArray = Array.from(peerAddressBytes);

      if (!Array.isArray(peerAddressBytesArray) || peerAddressBytesArray.length === 0) {
        throw new Error("Array conversion failed");
      }

    } catch (error) {
      throw new Error(`Failed to convert peer address bytes to array: ${String(error)}`);
    }

    try {

      // Create ExternalAddress from the peer address bytes
      // First create Bytes32 from the bytes, then create ExternalAddress from Bytes32
      // TODO: Get this dynamically from deployment config instead of hardcoding
      const wormholePackageId = "0xcb2d4a43e35a73835fa21ff381b105f699b8265355fab733bd7c5971f1caeeb0";

      const bytes32 = txb.moveCall({
        target: `${wormholePackageId}::bytes32::from_bytes`,
        arguments: [txb.pure.vector("u8", peerAddressBytesArray)],
      });

      const externalAddress = txb.moveCall({
        target: `${wormholePackageId}::external_address::new`,
        arguments: [bytes32],
      });

      txb.moveCall({
        target: `${this.packageId}::state::set_peer`,
        typeArguments: [this.contracts.token], // Use the token type from contracts
        arguments: [
          txb.object(this.adminCapId), // AdminCap
          txb.object(this.contracts.manager), // NTT state
          txb.pure.u16(wormholeChainId), // Chain ID
          externalAddress, // ExternalAddress object (properly created)
          txb.pure.u8(tokenDecimals), // Token decimals
          txb.pure.u64(inboundLimit.toString()), // Inbound limit
          txb.object("0x6"), // Clock object (standard Sui clock)
        ],
      });
    } catch (error) {
      throw new Error(`Failed to create setPeer transaction: ${error instanceof Error ? error.message : String(error)}`);
    }

    const unsignedTx = new SuiUnsignedTransaction(
      txb,
      this.network,
      this.chain,
      "Set Peer"
    );

    yield unsignedTx;
  }

  async getPeer<PC extends Chain>(chain: PC): Promise<Ntt.Peer<PC> | null> {
    const state = await this.provider.getObject({
      id: this.contracts.manager,
      options: {
        showContent: true,
      },
    });

    if (!state.data?.content || state.data.content.dataType !== "moveObject") {
      throw new Error("Failed to fetch NTT state object");
    }

    // const fields = (state.data.content as any).fields;
    // const peersTable = fields.peers.fields;

    // TODO: Convert chain name to chain ID and look up in peers table
    // The peers table is keyed by chain ID (u16)
    // We'd need a chain name to chain ID mapping

    // For now, return null indicating peer not found
    // In a full implementation:
    // const chainId = chainToChainId(chain);
    // const peerData = peersTable[chainId];
    // if (!peerData) return null;
    //
    // return {
    //   address: peerData.fields.address,
    //   tokenDecimals: peerData.fields.token_decimals,
    //   inboundLimit: BigInt(peerData.fields.inbound_rate_limit.fields.limit)
    // };

    return null;
  }

  async *setTransceiverPeer(
    ix: number,
    peer: ChainAddress,
    payer?: AccountAddress<C>
  ): AsyncGenerator<UnsignedTransaction<N, C>> {
    // Build transaction to set transceiver peer
    const txb = new Transaction();

    // TODO: This would call the transceiver's set_peer function
    // We need:
    // 1. Transceiver state object ID (based on ix)
    // 2. Admin cap for the transceiver
    // 3. Package ID for the transceiver contract

    // const chainId = chainToChainId(peer.chain);
    // const peerAddress = peer.address; // Convert to ExternalAddress

    // txb.moveCall({
    //   target: `${transceiverPackageId}::wormhole_transceiver::set_peer`,
    //   typeArguments: [managerAuthType],
    //   arguments: [
    //     transceiverAdminCap,
    //     transceiverState,
    //     chainId,
    //     peerAddress
    //   ]
    // });

    const unsignedTx = new SuiUnsignedTransaction(
      txb,
      this.network,
      this.chain,
      "Set Transceiver Peer"
    );

    yield unsignedTx;
  }

  // Transfer Methods
  async *transfer(
    sender: AccountAddress<C>,
    amount: bigint,
    destination: ChainAddress,
    options: Ntt.TransferOptions
  ): AsyncGenerator<UnsignedTransaction<N, C>> {

    if (!this.packageId) {
      throw new Error("Package ID required for transfer operation");
    }

    // Import chainToChainId from SDK
    const { chainToChainId } = await import("@wormhole-foundation/sdk-base");

    // Build the transaction for Sui transfer
    const txb = new Transaction();

    // Convert destination chain to wormhole chain ID
    const destinationChainId = chainToChainId(destination.chain);

    // Convert destination address to bytes
    let destinationAddressBytes: Uint8Array;
    try {
      if (typeof destination.address.toUint8Array === 'function') {
        destinationAddressBytes = destination.address.toUint8Array();
      } else if (typeof destination.address.toUniversalAddress === 'function') {
        const universalAddr = destination.address.toUniversalAddress();
        if (!universalAddr) {
          throw new Error("toUniversalAddress() returned null or undefined");
        }
        destinationAddressBytes = universalAddr.toUint8Array();
      } else {
        throw new Error(`destination.address does not have expected methods. Type: ${typeof destination.address}`);
      }
    } catch (error) {
      throw new Error(`Failed to convert destination address to bytes: ${error instanceof Error ? error.message : String(error)}`);
    }


    // Query the CoinMetadata object ID dynamically
    let coinMetadataId: string;
    if (this.contracts.token === "0x2::sui::SUI") {
      try {
        const coinMetadata = await this.provider.getCoinMetadata({ 
          coinType: this.contracts.token 
        });
        if (!coinMetadata?.id) {
          throw new Error("CoinMetadata not found for SUI");
        }
        coinMetadataId = coinMetadata.id;
      } catch (error) {
        throw new Error(`Failed to get CoinMetadata for ${this.contracts.token}: ${error instanceof Error ? error.message : String(error)}`);
      }
    } else {
      throw new Error(`Transfer not yet implemented for token: ${this.contracts.token}`);
    }


    // 1. Split coins from gas to get the required amount
    const coin = txb.splitCoins(txb.gas, [amount.toString()]);

    // 2. Create VersionGated object
    const versionGated = txb.moveCall({
      target: `${this.packageId}::upgrades::new_version_gated`,
      arguments: [],
    });

    // Since prepare_transfer returns a tuple (TransferTicket, Balance), we need to properly
    // extract the individual elements. In Sui's transaction builder, we can access tuple elements
    // using array-like indexing on the result.
    const prepareResult = txb.moveCall({
      target: `${this.packageId}::ntt::prepare_transfer`,
      typeArguments: [this.contracts.token],
      arguments: [
        txb.object(this.contracts.manager), // state
        coin, // coins
        txb.object(coinMetadataId), // coin_meta
        txb.pure.u16(destinationChainId), // recipient_chain
        txb.pure.vector("u8", Array.from(destinationAddressBytes)), // recipient (as vector<u8>)
        txb.pure.option("vector<u8>", null), // payload (no payload for now)
        txb.pure.bool(options.queue || false), // should_queue
      ],
    });


    // Extract the TransferTicket (first element) from the tuple result
    // Use type assertions to bypass TypeScript's strict checking for tuple access
    const ticket = (prepareResult as any)[0];
    // const dust = (prepareResult as any)[1]; // Not using dust for now


    // Now call transfer_tx_sender with just the ticket
    txb.moveCall({
      target: `${this.packageId}::ntt::transfer_tx_sender`,
      typeArguments: [this.contracts.token],
      arguments: [
        txb.object(this.contracts.manager), // state (mutable)
        versionGated, // version_gated
        txb.object(coinMetadataId), // coin_meta
        ticket as any, // Just the TransferTicket from the tuple
        txb.object("0x6"), // clock (standard Sui clock)
      ],
    });



    // Note: For simplicity, we're not handling the dust balance for now
    // In a production implementation, you would want to handle the dust by:
    // - Converting the Balance to a Coin using coin::from_balance
    // - Transferring it back to the sender or handling it appropriately


    const unsignedTx = new SuiUnsignedTransaction(
      txb,
      this.network,
      this.chain,
      "NTT Transfer"
    );

    yield unsignedTx;
  }

  async *redeem(attestations: Ntt.Attestation[]): AsyncGenerator<UnsignedTransaction<N, C>> {
    // Build transaction to redeem attestations
    const txb = new Transaction();

    // TODO: This would call ntt::redeem for each attestation
    // We need:
    // 1. NTT state object ID (this.contracts.manager)
    // 2. Coin metadata object ID
    // 3. Clock object ID (usually 0x6)
    // 4. Package ID for the NTT contracts
    // 5. Validated transceiver messages from attestations

    // For each attestation:
    // const validatedMessage = parseAttestation(attestation);
    //
    // txb.moveCall({
    //   target: `${nttPackageId}::ntt::redeem`,
    //   typeArguments: [tokenType, transceiverType],
    //   arguments: [
    //     state,
    //     versionGated,
    //     coinMetadata,
    //     validatedMessage,
    //     clock
    //   ]
    // });

    const unsignedTx = new SuiUnsignedTransaction(
      txb,
      this.network,
      this.chain,
      "Redeem NTT Transfer"
    );

    yield unsignedTx;
  }

  async quoteDeliveryPrice(
    destination: Chain,
    options: Ntt.TransferOptions
  ): Promise<bigint> {
    throw new Error("Not implemented");
  }

  async isRelayingAvailable(destination: Chain): Promise<boolean> {
    return false;
  }

  // Rate Limiting
  async getCurrentOutboundCapacity(): Promise<bigint> {
    const state = await this.provider.getObject({
      id: this.contracts.manager,
      options: {
        showContent: true,
      },
    });

    if (!state.data?.content || state.data.content.dataType !== "moveObject") {
      throw new Error("Failed to fetch NTT state object");
    }

    const fields = (state.data.content as any).fields;
    const outboxRateLimit = fields.outbox.fields.rate_limit.fields;

    // Get current timestamp (this would ideally come from Clock object)
    const currentTime = Date.now();

    // Calculate capacity using the rate limit formula
    // This is a simplified version - in practice we'd need the exact formula from Move
    const limit = BigInt(outboxRateLimit.limit);
    const capacityAtLastTx = BigInt(outboxRateLimit.capacity_at_last_tx);
    const lastTxTimestamp = BigInt(outboxRateLimit.last_tx_timestamp);

    // Simplified capacity calculation
    const timePassed = BigInt(currentTime) - lastTxTimestamp;
    const rateLimitDuration = BigInt(24 * 60 * 60 * 1000); // 24 hours in ms

    const additionalCapacity = (timePassed * limit) / rateLimitDuration;
    const currentCapacity = capacityAtLastTx + additionalCapacity;

    return currentCapacity > limit ? limit : currentCapacity;
  }

  async getOutboundLimit(): Promise<bigint> {
    const state = await this.provider.getObject({
      id: this.contracts.manager,
      options: {
        showContent: true,
      },
    });

    if (!state.data?.content || state.data.content.dataType !== "moveObject") {
      throw new Error("Failed to fetch NTT state object");
    }

    const fields = (state.data.content as any).fields;
    const outboxRateLimit = fields.outbox.fields.rate_limit.fields;

    return BigInt(outboxRateLimit.limit);
  }

  async *setOutboundLimit(limit: bigint, payer?: AccountAddress<C>): AsyncGenerator<UnsignedTransaction<N, C>> {
    // Build transaction to set outbound limit
    const txb = new Transaction();

    // TODO: This would call state::set_outbound_rate_limit
    // We need:
    // 1. AdminCap object ID
    // 2. NTT state object ID (this.contracts.manager)
    // 3. Clock object ID (usually 0x6)
    // 4. Package ID for the NTT contracts

    // txb.moveCall({
    //   target: `${nttPackageId}::state::set_outbound_rate_limit`,
    //   typeArguments: [tokenType],
    //   arguments: [
    //     adminCap,
    //     state,
    //     limit.toString(),
    //     clock
    //   ]
    // });

    const unsignedTx = new SuiUnsignedTransaction(
      txb,
      this.network,
      this.chain,
      "Set Outbound Limit"
    );

    yield unsignedTx;
  }

  async getCurrentInboundCapacity<PC extends Chain>(fromChain: PC): Promise<bigint> {
    throw new Error("Not implemented");
  }

  async getInboundLimit<PC extends Chain>(fromChain: PC): Promise<bigint> {
    throw new Error("Not implemented");
  }

  async *setInboundLimit<PC extends Chain>(
    fromChain: PC,
    limit: bigint,
    payer?: AccountAddress<C>
  ): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  async getRateLimitDuration(): Promise<bigint> {
    // Rate limit duration is a constant in the Move contract
    // 24 hours in milliseconds
    return BigInt(24 * 60 * 60 * 1000);
  }

  // Transfer Status
  async getIsApproved(attestation: Ntt.Attestation): Promise<boolean> {
    // In Sui, approval status would be checked by looking at the inbox item
    // and checking if it has enough votes (>= threshold)
    // This requires parsing the attestation to get the message details
    // and looking it up in the inbox table

    // For now, return false as we'd need to:
    // 1. Parse the attestation to get chain ID and message
    // 2. Query the inbox table with the InboxKey
    // 3. Check if votes >= threshold
    return false;
  }

  async getIsExecuted(attestation: Ntt.Attestation): Promise<boolean> {
    // In Sui, execution status would be checked by looking at the inbox item's
    // release_status field to see if it's ReleaseStatus::Released

    // For now, return false as we'd need to:
    // 1. Parse the attestation to get chain ID and message
    // 2. Query the inbox table with the InboxKey
    // 3. Check if release_status is Released
    return false;
  }

  async getIsTransferInboundQueued(attestation: Ntt.Attestation): Promise<boolean> {
    // In Sui, queued status would be checked by looking at the inbox item's
    // release_status field to see if it's ReleaseStatus::ReleaseAfter(timestamp)

    // For now, return false as we'd need to:
    // 1. Parse the attestation to get chain ID and message
    // 2. Query the inbox table with the InboxKey
    // 3. Check if release_status is ReleaseAfter
    return false;
  }

  async getInboundQueuedTransfer<PC extends Chain>(
    fromChain: PC,
    transceiverMessage: Ntt.Message
  ): Promise<Ntt.InboundQueuedTransfer<C> | null> {
    throw new Error("Not implemented");
  }

  async *completeInboundQueuedTransfer<PC extends Chain>(
    fromChain: PC,
    transceiverMessage: Ntt.Message,
    payer?: AccountAddress<C>
  ): AsyncGenerator<UnsignedTransaction<N, C>> {
    throw new Error("Not implemented");
  }

  // Transceiver Management
  async getTransceiver(ix: number): Promise<NttTransceiver<N, C, Ntt.Attestation> | null> {
    // In Sui, transceivers are registered in the TransceiverRegistry
    // We would need to:
    // 1. Query the registry to get the transceiver at index ix
    // 2. Create a transceiver instance based on the type

    // For now, return null indicating transceiver not found
    // In a full implementation:
    // const transceiverInfo = await getTransceiverAtIndex(ix);
    // if (transceiverInfo.type === "wormhole") {
    //   return new SuiWormholeTransceiver(...);
    // }
    return null;
  }

  async verifyAddresses(): Promise<Partial<Ntt.Contracts> | null> {
    // Verify that the addresses in the contracts configuration are valid
    try {
      // Check if manager address exists and is a valid NTT state object
      const state = await this.provider.getObject({
        id: this.contracts.manager,
        options: { showContent: true },
      });

      if (!state.data?.content || state.data.content.dataType !== "moveObject") {
        return null;
      }

      // Return the verified addresses
      return {
        manager: this.contracts.manager,
        transceiver: this.contracts.transceiver,
      };
    } catch {
      return null;
    }
  }
}
