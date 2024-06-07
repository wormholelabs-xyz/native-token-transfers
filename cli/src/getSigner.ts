import solana from "@wormhole-foundation/sdk/platforms/solana";
import * as myEvmSigner from "./evmsigner.js";
import { ChainContext, Wormhole, chainToPlatform, type Chain, type ChainAddress, type Network, type Signer } from "@wormhole-foundation/sdk";

// TODO: copied these from the examples. do they exist in the sdk?
export interface SignerStuff<N extends Network, C extends Chain> {
    chain: ChainContext<N, C>;
    signer: Signer<N, C>;
    address: ChainAddress<C>;
}


export async function getSigner<N extends Network, C extends Chain>(
    chain: ChainContext<N, C>,
    privateKey?: string
): Promise<SignerStuff<N, C>> {
    let signer: Signer;
    const platform = chainToPlatform(chain.chain);
    switch (platform) {
        case "Solana":
            privateKey = privateKey ?? process.env.SOLANA_PRIVATE_KEY;
            if (privateKey === undefined) {
                throw new Error("SOLANA_PRIVATE_KEY env var not set");
            }
            signer = await solana.getSigner(
                await chain.getRpc(),
                privateKey,
                { debug: false }
            );
            break;
        case "Evm":
            privateKey = privateKey ?? process.env.ETH_PRIVATE_KEY;
            if (privateKey === undefined) {
                throw new Error("ETH_PRIVATE_KEY env var not set");
            }
            signer = await myEvmSigner.getEvmSigner(
                await chain.getRpc(),
                privateKey,
                { debug: true }
            );
            // signer = await evm.getSigner(
            //     await chain.getRpc(),
            //     privateKey,
            //     { debug: true }
            // );
            // signer = await
            break;
        default:
            throw new Error("Unrecognized platform: " + platform);
    }

    return {
        chain,
        signer: signer as Signer<N, C>,
        address: Wormhole.chainAddress(chain.chain, signer.address()),
    };
}
