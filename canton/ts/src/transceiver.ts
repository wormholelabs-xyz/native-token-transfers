import type { Chain, Network } from "@wormhole-foundation/sdk-base";
import type {
  AccountAddress,
  ChainAddress,
} from "@wormhole-foundation/sdk-definitions";
import type {
  Ntt,
  NttTransceiver,
} from "@wormhole-foundation/sdk-definitions-ntt";
import type {
  CantonChains,
  CantonUnsignedTransaction,
} from "@wormhole-foundation/sdk-canton";
import { CantonAddress } from "@wormhole-foundation/sdk-canton";
import type { CantonNtt } from "./ntt.js";

/**
 * The Wormhole transceiver of a Canton NTT deployment: the transceiver Emitter, owned and
 * registered by guardianGovernance. Its peers are part of the manager's peer table, set together
 * with the peer manager by SetPeer.
 */
export class CantonNttWormholeTransceiver<
  N extends Network,
  C extends CantonChains,
> implements NttTransceiver<N, C, Ntt.Attestation>
{
  constructor(readonly manager: CantonNtt<N, C>) {}

  async getTransceiverType(_payer?: AccountAddress<C>): Promise<string> {
    return "wormhole";
  }

  /** The configured transceiver (emitter) address */
  async getAddress(): Promise<ChainAddress<C>> {
    return {
      chain: this.manager.chain,
      address: new CantonAddress(this.manager.transceiverAddress()),
    } as unknown as ChainAddress<C>;
  }

  async getPeer<PC extends Chain>(chain: PC): Promise<ChainAddress<PC> | null> {
    return this.manager.getTransceiverPeer(0, chain);
  }

  /**
   * Changes the peer transceiver of a chain that already has a peer (CantonNtt.setTransceiverPeer):
   * Canton's SetPeer sets manager, transceiver and decimals together.
   */
  async *setPeer(
    peer: ChainAddress<Chain>,
    payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    yield* this.manager.setTransceiverPeer(0, peer, payer);
  }

  async *setPauser(
    _newPauser: AccountAddress<C>,
    _payer?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    throw new Error(
      "CantonNttWormholeTransceiver.setPauser is unsupported: Canton NTT has no pauser role"
    );
  }

  async getPauser(): Promise<AccountAddress<C> | null> {
    return null;
  }

  /** Redeems the attestation through the manager (Release or Mint), with the sender as executor */
  async *receive(
    attestation: Ntt.Attestation,
    sender?: AccountAddress<C>
  ): AsyncGenerator<CantonUnsignedTransaction<N, C>> {
    yield* this.manager.redeem([attestation], sender);
  }
}
