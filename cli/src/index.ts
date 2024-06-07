#!/usr/bin/env bun
import "./side-effects";
import evm from "@wormhole-foundation/sdk/platforms/evm";
import solana from "@wormhole-foundation/sdk/platforms/solana";
import { encoding } from '@wormhole-foundation/sdk-connect';

import chalk from "chalk";
import yargs from "yargs";
import { $ } from "bun";
import { hideBin } from "yargs/helpers";
import { Connection, Keypair, PublicKey, Transaction } from "@solana/web3.js";
import fs from "fs";
import readline from "readline";
import { BN } from "@coral-xyz/anchor";
import * as spl from "@solana/spl-token";
import {
    deserialize,
} from '@wormhole-foundation/sdk-definitions'
import { ChainContext, UniversalAddress, Wormhole, assertChain, canonicalAddress, chainToPlatform, chains, isNetwork, networks, signSendWait, toUniversal, type Chain, type ChainAddress, type ConfigOverrides, type Network, type Signer } from "@wormhole-foundation/sdk";
import "@wormhole-foundation/sdk-evm-ntt";
import "@wormhole-foundation/sdk-solana-ntt";
import "@wormhole-foundation/sdk-definitions-ntt";
import type { Ntt, NttTransceiver } from "@wormhole-foundation/sdk-definitions-ntt";

import { type SolanaChains } from "@wormhole-foundation/sdk-solana";

import { colorizeDiff, diffObjects } from "./diff";
import { getSigner } from "./getSigner";
import { NTT, SolanaNtt } from "@wormhole-foundation/sdk-solana-ntt";
// import {NTT as SolanaNTT, getNttProgram} from "@wormhole-foundation/sdk-solana-ntt";

// TODO: check if manager can mint the token in burning mode (on solana it's
// simple. on evm we need to simulate with prank)

// TODO: interactively build json file
// 1. get the network if not provided
// 2. add chain to config
// 3. figure out if the contract is deployed
//   a) if it is, see if it's initialized
//     i) if it is, see if it's the same as the config (allowing fixing the config, or fixing the contract)
//    ii) if it's not, initialize it according to config (in this case prompt for the fields)
//   b) if it's not, deploy it, and initialize it according to config (in this case prompt for the fields)
// 4. if the contract is deployed, see if the peers are registered
//  a) if they are, see if they match the config
//  b) if they are not, register them according to the config
//
// for actual contract deployment, we need to be able to:
// 1. build the contract
// 2. deploy the contract
// 3. initialize the contract
//
// additionally, for transsactions that need to be executed against the
// contract, we need the corresponding private key. this is the owner as is set
// on the contract (when making changes) or the owner in the config file (when deploying)
//
// for this, the deployment file should have a path to the binary, and should be
// able to pull the bytecode and match against the local binary.
// for solana, it should also be possible to verify the hardcoded addresses (using xxd)
//
// TODO: adopt other config verification stuff from Max's script (more chain specific)
//
// $ ntt init Testnet # should create a new deployment file with default limits and no chains
// $ ntt add-chain Solana

const overrides = {
    chains: {
        Bsc: {
            // rpc: "https://bsc-testnet-rpc.publicnode.com"
            rpc: "http://127.0.0.1:8545"
        },
        Sepolia: {
            // rpc: "wss://ethereum-sepolia-rpc.publicnode.com"
            rpc: "http://127.0.0.1:8546"
        },
        // Solana: {
        //     rpc: "http://127.0.0.1:8899"
        // }
    }
}

export type Deployment<C extends Chain> = {
    ctx: ChainContext<Network, C>,
    ntt: Ntt<Network, C>,
    whTransceiver: NttTransceiver<Network, C, Ntt.Attestation>,
    decimals: number,
    manager: ChainAddress<C>,
    config: {
        remote?: ChainConfig,
        local?: ChainConfig,
    },
}

// TODO: rename
export type ChainConfig = {
    mode: Ntt.Mode,
    paused: boolean,
    owner: string,
    manager: string,
    token: string,
    transceivers: {
        threshold: number,
        wormhole: string,
    },
    limits: {
        outbound: string,
        inbound: Partial<{ [C in Chain]: string }>,
        // TOOD: figure out a good way to represent inbound limits
        // 1. a single number
        // 2. a mapping of chains to numbers
        // 3. a combination of the two
    }
}

export type Config = {
    network: Network,
    chains: Partial<{
        [C in Chain]: ChainConfig
    }>,
    defaultLimits?: {
        outbound: string,
    }
}

export const NETWORK_OPTIONS = {
    alias: "n",
    describe: "Network",
    choices: networks,
    demandOption: true,
} as const;

yargs(hideBin(process.argv))
    .scriptName("ntt")
    .command("to-hex",
        "base58 to hex",
        (yargs) => yargs.option("address", {
            alias: "a",
            describe: "Address",
            demandOption: true,
            type: "string",
        }),
        (argv) => {
            const address = new PublicKey(argv["address"]);
            console.log(`0x${address.toBuffer().toString("hex")}`);
        })
    .command("new <path>",
        "create a new NTT project",
        (yargs) => yargs
            .positional("path", {
                describe: "Path to the project",
                type: "string",
                demandOption: true,
            }),
        async (argv) => {
            // NOTE: would be nice to silence the output of this, but it looks
            // like calling .text() or .quiet() doesn't work (the process just halts).
            // only seems to affect "git" and no other processes.
            // until that's fixed, the command prints "true" (or "false") to the
            // console.. sigh
            const git = await $`git rev-parse --is-inside-work-tree`.nothrow();
            if (git.stdout.toString().trim() === "true") {
                console.error("Already in a git repository");
                process.exit(1);
            }
            const path = argv["path"];
            await $`git clone git@github.com:wormhole-foundation/example-native-token-transfers.git ${path} --recurse-submodules`;
        })
    .command("add-chain <chain>",
        "add a chain to the deployment file",
        (yargs) => yargs
            .positional("chain", {
                describe: "Chain",
                type: "string",
                choices: chains,
                demandOption: true,
            })
            // TODO: add ability to specify manager address (then just pull the config)
            // .option("manager", {
            //     describe: "Manager address",
            //     type: "string",
            // })
            .option("program-key", {
                describe: "Path to program key json (Solana)",
                type: "string",
            })
            .option("payer", {
                describe: "Path to payer key json (Solana)",
                type: "string",
            })
            .option("binary", {
                describe: "Path to program binary (.so file -- Solana)",
                type: "string",
            })
            .option("token", {
                describe: "Token address",
                type: "string",
            })
            .option("mode", {
                alias: "m",
                describe: "Mode",
                type: "string",
                choices: ["locking", "burning"],
            })
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            }),
        async (argv) => {
            const path = argv["path"];
            const deployments: Config = JSON.parse(fs.readFileSync(path).toString());
            const chain: Chain = argv["chain"];
            let mode = argv["mode"] as Ntt.Mode | undefined;
            const token = argv["token"];
            const network = deployments.network as Network;

            if (chain in deployments.chains) {
                console.error(`Chain ${chain} already exists in ${path}`);
                process.exit(1);
            }

            const existsLocking = Object.values(deployments.chains).some((c) => c.mode === "locking");

            if (existsLocking) {
                if (mode && mode === "locking") {
                    console.error("Only one locking chain is allowed");
                    process.exit(1);
                }
                mode = "burning";
            }

            if (!mode) {
                console.error("Mode is required (use --mode)");
                process.exit(1);
            }

            if (!token) {
                console.error("Token is required (use --token)");
                process.exit(1);
            }

            // let's deploy

            // TODO: factor out to function to get chain context
            const wh = new Wormhole(network, [solana.Platform, evm.Platform], overrides);
            const ch = wh.getChain(chain);

            // TODO: make manager configurable
            const deployedManager = await deploy(mode, ch, token, argv["payer"], argv["program-key"], argv["binary"]);

            const [config, _ctx, _ntt, decimals] =
                await pullChainConfig(network, deployedManager, overrides);

            console.log("token decimals:", chalk.yellow(decimals));

            deployments.chains[chain] = config;
            fs.writeFileSync(path, JSON.stringify(deployments, null, 2));
            console.log(`Added ${chain} to ${path}`);
        })
    .command("clone <network> <chain> <address>",
        "initialize a deployment file from an existing contract",
        (yargs) => yargs
            .positional("network", {
                describe: "Network",
                choices: networks,
                demandOption: true,
            })
            .positional("chain", {
                describe: "Chain",
                type: "string",
                choices: chains,
                demandOption: true,
            })
            .positional("address", {
                describe: "Address",
                type: "string",
                demandOption: true,
            })
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            })
            .option("verbose", {
                alias: "v",
                describe: "Verbose output",
                type: "boolean",
                default: false,
            }),
        async (argv) => {
            if (!isNetwork(argv["network"])) {
                console.error("Invalid network");
                process.exit(1);
            }

            const path = argv["path"];
            const verbose = argv["verbose"];
            // check if the file exists
            if (fs.existsSync(path)) {
                console.error(`Deployment file already exists at ${path}`);
                process.exit(1);
            }

            // step 1. grab the config
            // step 2. discover registrations
            // step 3. grab registered peer configs
            // step 4 (?). recursively clone their peers

            const chain = argv["chain"];
            assertChain(chain)

            const manager = argv["address"];
            const network = argv["network"];

            const universalManager = toUniversal(chain, manager);

            const ntts: Partial<{ [C in Chain]: Ntt<Network, C> }> = {};

            const [config, _ctx, ntt, _decimals] =
                await pullChainConfig(network, { chain, address: universalManager }, overrides);

            ntts[chain] = ntt as any;

            const configs: Partial<{ [C in Chain]: ChainConfig }> = {
                [chain]: config,
            }

            // discover peers
            let count = 0;
            for (const c of chains) {
                process.stdout.write(`[${count}/${chains.length - 1}] Fetching peer config for ${c}`);
                await new Promise((resolve) => setTimeout(resolve, 100));
                count++;

                let peer: Ntt.Peer<Chain> | null = null;

                // getPeer might be rate limited. if it is, we wait a bit
                while (true) {
                    try {
                        peer = await ntt.getPeer(c);
                        break;
                    } catch (e) {
                        // @ts-ignore TODO
                        if (e.toString().includes("limit") || (JSON.stringify(e)).includes("limit")) {
                            process.stdout.write(` (rate limited, waiting...)`);
                            await new Promise((resolve) => setTimeout(resolve, 5000));
                            continue;
                        } else {
                            throw e;
                        }
                    }
                }
                process.stdout.write(`\r`);
                if (peer === null) {
                    continue;
                }
                // TODO: I "know" these are universal addresses, but is there a
                // way to convert them in case they're not?
                const address: UniversalAddress = peer.address.address as UniversalAddress;
                const [peerConfig, _ctx, peerNtt] = await pullChainConfig(network, { chain: c, address }, overrides);
                ntts[c] = peerNtt as any;
                configs[c] = peerConfig;
            }

            // sort chains by name
            const sorted = Object.fromEntries(Object.entries(configs).sort(([a], [b]) => a.localeCompare(b)));

            // sleep for a bit to avoid rate limiting when making the getDecimals call
            await new Promise((resolve) => setTimeout(resolve, 2000));

            // now loop through the chains, and query their peer information to get the inbound limits
            await pullInboundLimits(ntts, sorted, verbose)

            const deployment: Config = {
                network: argv["network"],
                chains: sorted,
            };
            fs.writeFileSync(path, JSON.stringify(deployment, null, 2));
        })
    .command("init <network>",
        "initialize a deployment file",
        (yargs) => yargs
            .positional("network", {
                describe: "Network",
                choices: networks,
                demandOption: true,
            })
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            }),
        async (argv) => {
            if (!isNetwork(argv["network"])) {
                console.error("Invalid network");
                process.exit(1);
            }
            const deployment = {
                network: argv["network"],
                chains: {},
            };
            const path = argv["path"];
            // check if the file exists
            if (fs.existsSync(path)) {
                console.error(`Deployment file already exists at ${path}`);
                process.exit(1);
            }
            fs.writeFileSync(path, JSON.stringify(deployment, null, 2));
        })
    .command("pull",
        "pull the remote configuration",
        (yargs) => yargs
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            })
            .option("verbose", {
                alias: "v",
                describe: "Verbose output",
                type: "boolean",
                default: false,
            }),
        async (argv) => {
            const deployments: Config = JSON.parse(fs.readFileSync(argv["path"]).toString());
            const verbose = argv["verbose"];
            const network = deployments.network as Network;
            const path = argv["path"];
            const deps: Partial<{ [C in Chain]: Deployment<C> }> = await pullDeployments(deployments, network, verbose);

            let changed = false;
            for (const [chain, deployment] of Object.entries(deps)) {
                assertChain(chain);
                const diff = diffObjects(deployments.chains[chain]!, deployment.config.remote!);
                if (Object.keys(diff).length !== 0) {
                    console.error(chalk.reset(colorizeDiff({ [chain]: diff })));
                    changed = true;
                    deployments.chains[chain] = deployment.config.remote!
                }
            }
            if (!changed) {
                console.log(`${path} is already up to date`);
                process.exit(0);
            }

            await askForConfirmation();
            fs.writeFileSync(path, JSON.stringify(deployments, null, 2));
            console.log(`Updated ${path}`);
        })
    .command("push",
        "push the local configuration",
        (yargs) => yargs
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            })
            .option("verbose", {
                alias: "v",
                describe: "Verbose output",
                type: "boolean",
                default: false,
            }),
        async (argv) => {
            const deployments: Config = JSON.parse(fs.readFileSync(argv["path"]).toString());
            const verbose = argv["verbose"];
            const network = deployments.network as Network;
            const deps: Partial<{ [C in Chain]: Deployment<C> }> = await pullDeployments(deployments, network, verbose);

            const missing = await missingPeers(deps, verbose);

            // push missing peers
            for (const [chain, peers] of Object.entries(missing)) {
                assertChain(chain);
                const ntt = deps[chain]!.ntt;
                const ctx = deps[chain]!.ctx;
                const signer = await getSigner(ctx)
                for (const manager of peers.managerPeers) {
                    const tx = ntt.setPeer(manager.address, manager.tokenDecimals, manager.inboundLimit, signer.address.address)
                    await signSendWait(ctx, tx, signer.signer)
                }
                for (const transceiver of peers.transceiverPeers) {
                    const tx = ntt.setWormholeTransceiverPeer(transceiver, signer.address.address)
                    await signSendWait(ctx, tx, signer.signer)
                }
            }

            // pull deps again
            const depsAfterRegistrations: Partial<{ [C in Chain]: Deployment<C> }> = await pullDeployments(deployments, network, verbose);

            for (const [chain, deployment] of Object.entries(depsAfterRegistrations)) {
                assertChain(chain);
                await pushDeployment(deployment as any);
            }
        })
    .command("status",
        "check the status of the deployment",
        (yargs) => yargs
            .option("path", {
                alias: "p",
                describe: "Path to the deployment file",
                default: "deployment.json",
                type: "string",
            })
            .option("verbose", {
                alias: "v",
                describe: "Verbose output",
                type: "boolean",
                default: false,
            }),
        async (argv) => {
            const path = argv["path"];
            const verbose = argv["verbose"];
            // TODO: I don't like the variable names here
            const deployments: Config = JSON.parse(fs.readFileSync(path).toString());

            const network = deployments.network as Network;

            let deps: Partial<{
                [C in Chain]: Deployment<C>;
            }> = await pullDeployments(deployments, network, verbose);

            let errors = 0;

            // diff remote and local configs
            for (const [chain, deployment] of Object.entries(deps)) {
                const local = deployment.config.local;
                const remote = deployment.config.remote;
                const a = { [chain]: local! };
                const b = { [chain]: remote! };

                const diff = diffObjects(a, b);
                if (Object.keys(diff).length !== 0) {
                    console.error(chalk.reset(colorizeDiff(diff)));
                    errors++;
                }
            }

            // verify peers
            const missing = await missingPeers(deps, verbose);

            if (Object.keys(missing).length > 0) {
                errors++;
            }

            for (const [chain, peers] of Object.entries(missing)) {
                console.error(`Missing peers for ${chain}:`);
                const managers = peers.managerPeers;
                const transceivers = peers.transceiverPeers;
                for (const manager of managers) {
                    console.error(`  Manager: ${manager.address.chain}`);
                }
                for (const transceiver of transceivers) {
                    console.error(`  Transceiver: ${transceiver.chain}`);
                }
            }

            if (errors > 0) {
                console.error("Run `ntt pull` to pull the remote configuration (overwriting the local one)");
                console.error("Run `ntt push` to push the local configuration (overwriting the remote one) by executing the necessary transactions");
                process.exit(1);
            } else {
                console.log(`${path} is up to date with the on-chain configuration.`);
                process.exit(0);
            }
        })
    .command("solana",
        "Solana commands",
        (yargs) => {
            yargs
                .command("deploy",
                    "deploy the solana program",
                    (yargs) => yargs.option("network", NETWORK_OPTIONS),
                    (argv) => {
                        throw new Error("Not implemented");
                    })
                .command("transfer",
                    "transfer tokens",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("to", {
                            describe: "Recipient",
                            demandOption: true,
                            type: "string",
                        })
                        .option("chain", {
                            describe: "Chain",
                            demandOption: true,
                            type: "string",
                        })
                        .option("amount", {
                            describe: "Amount",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));
                        const recipient = Buffer.from(argv["to"], "hex");
                        if (recipient.length !== 32) {
                            throw new Error("Invalid recipient (must be 32 bytes)");
                        }
                        const amount = new BN(argv["amount"]);

                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        const config = await ntt.getConfig();
                        // TODO: add this getter to the SDK
                        const mint = await spl.getMint(connection, config.mint, undefined, config.tokenProgram);

                        // TODO: allow specifying the token account?
                        const ata = spl.getAssociatedTokenAddressSync(
                            config.mint,
                            payer.publicKey,
                            true,
                            config.tokenProgram
                        );

                        console.log(`pubkey: ${payer.publicKey.toBase58()}`);
                        console.log(`mint: ${config.mint.toBase58()}`)
                        console.log(`ATA: ${ata.toBase58()}`);
                        console.log(`token program: ${config.tokenProgram.toBase58()}`);

                        const balance = await spl.getAccount(connection, ata, undefined, config.tokenProgram);

                        const decimals = mint.decimals;
                        // TODO: this can't amounts larger than 2^53-1
                        const adjusted = amount.toNumber() / Math.pow(10, decimals);

                        const adjustedBalance = Number(balance.amount) / Math.pow(10, decimals);

                        if (new BN(balance.amount.toString()) < amount) {
                            console.error(`Insufficient balance. Adjusted balance: ${adjustedBalance} tokens`);
                            process.exit(1);
                        }

                        console.log(`Transferring ${adjusted} tokens (balance: ${adjustedBalance} tokens) to 0x${recipient.toString("hex")} on ${argv["chain"]}`);
                        await askForConfirmation();

                        await ntt.transfer({
                            payer,
                            from: ata,
                            fromAuthority: payer,
                            amount,
                            recipientChain: argv["chain"] as any, // TODO: version mismatch? should be able to type assert
                            recipientAddress: recipient,
                            shouldQueue: false
                        });


                    })
                .command("redeem",
                    "redeem transaction",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("vaa", {
                            describe: "VAA",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));

                        const vaaBuf = Buffer.from(argv["vaa"], "hex");
                        const wormholeNTT = deserialize('Ntt:WormholeTransfer', argv.vaa)
                        const recipientNTT = wormholeNTT.payload.recipientNttManager;
                        const recipientChain = wormholeNTT.payload.nttManagerPayload.payload.recipientChain;
                        if (recipientChain !== "Solana") {
                            throw new Error("Unsupported destination chain");
                        }

                        const ntt = new NTT(connection, {
                            // nttId: recipientNTT.toNative("Solana"), // TODO: couldn't figure out how to import the platform
                            nttId: new PublicKey(recipientNTT.toUint8Array()),
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        console.log("Posting VAA");
                        // TODO: check if VAA is already posted
                        await postVaa(connection, payer, vaaBuf, new PublicKey("3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5"));

                        console.log("Redeeming");
                        // create associated token account

                        const config = await ntt.getConfig();

                        await spl.createAssociatedTokenAccountIdempotent(
                            connection,
                            payer,
                            config.mint,
                            new PublicKey(wormholeNTT.payload.nttManagerPayload.payload.recipientAddress.toUint8Array()),
                            undefined,
                            config.tokenProgram
                        )

                        await ntt.redeem({
                            payer,
                            vaa: vaaBuf,
                        })
                    })
                .command("set-outbound-rate-limit",
                    "set outbound rate limit",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("limit", {
                            describe: "Rate limit",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));
                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        const limit = new BN(argv["limit"]);
                        const config = await ntt.getConfig();

                        const mint = await spl.getMint(connection, config.mint, undefined, config.tokenProgram);
                        const decimals = mint.decimals;

                        const adjusted = limit.div(new BN(10).pow(new BN(decimals)));

                        console.log(`Setting outbound rate limit to ${adjusted} tokens`);
                        await askForConfirmation();

                        await ntt.setOutboundLimit({
                            owner: payer,
                            chain: undefined, // TODO: remove
                            limit,
                        })
                    })
                .command(
                    "info",
                    "info of solana NTT",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        const version = await ntt.version(new PublicKey("PAytVxxSUkQDDT69XG2mECPixpMAzQ7hg9gm2pmdFKu"));

                        console.log(`program address: ${ntt.program.programId.toBase58()}, 0x${ntt.program.programId.toBuffer().toString("hex")}`);

                        const config = await ntt.getConfig();
                        const outboundRateLimit = await ntt.program.account.outboxRateLimit.fetch(ntt.outboxRateLimitAccountAddress());
                        config.outboundRateLimit = outboundRateLimit.rateLimit;
                        config.version = version;
                        console.log(JSON.stringify(config, null, 2));

                        const emitterAccount = ntt.emitterAccountAddress();

                        console.log(`Emitter account (transceiver peer): ${emitterAccount.toBase58()}, 0x${emitterAccount.toBuffer().toString("hex")}`);

                        const sepoliaPeerAddress = ntt.peerAccountAddress("sepolia");
                        const peer = await ntt.program.account.nttManagerPeer.fetch(sepoliaPeerAddress);
                        peer.address = Buffer.from(peer.address).toString("hex");
                        const inboudRateLimit = await ntt.program.account.inboxRateLimit.fetch(ntt.inboxRateLimitAccountAddress("sepolia"));
                        peer.inboundRateLimit = inboudRateLimit.rateLimit;
                        console.log("Sepolia peer:")
                        console.log(JSON.stringify(peer, null, 2));

                        const sepoliaWormholeTransceiverPeerAddress = ntt.transceiverPeerAccountAddress("sepolia");
                        const transceiverPeer = await ntt.program.account.transceiverPeer.fetch(sepoliaWormholeTransceiverPeerAddress);
                        console.log("Sepolia wormhole transceiver peer:")
                        transceiverPeer.address = Buffer.from(transceiverPeer.address).toString("hex");
                        console.log(JSON.stringify(transceiverPeer, null, 2));
                    })
                .command("initialize",
                    "initialize the NTT program",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("mode", {
                            alias: "m",
                            describe: "Mode",
                            demandOption: true,
                            choices: ["locking", "burning"],
                            type: "string",
                        })
                        .option("mint", {
                            describe: "Mint",
                            demandOption: true,
                            type: "string",
                        })
                        .option("outbound-limit", {
                            alias: "o",
                            describe: "Outbound limit",
                            demandOption: true,
                            type: "string",
                        })
                    ,
                    async (argv) => {
                        const connection = solanaConnection(argv.network);

                        const deployed = connection.getAccountInfo(new PublicKey(argv["ntt-address"]));
                        if (deployed === null) {
                            throw new Error("NTT program not deployed");
                        }

                        // TODO: this only works for devnet (remove hardcode)
                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        // TODO: make idempotent (and verify stuff)
                        const initialized = await connection.getAccountInfo(ntt.configAccountAddress());
                        if (initialized !== null) {
                            console.log("NTT program already initialized. Skipping.");
                            return;
                        }

                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));

                        const mintAccount = await connection.getAccountInfo(new PublicKey(argv["mint"]));
                        if (mintAccount === null) {
                            throw new Error("Mint account not found");
                        }
                        const tokenProgram = mintAccount.owner;

                        const mint = await spl.getMint(connection, new PublicKey(argv["mint"]), undefined, tokenProgram);
                        const outboundLimit = new BN(argv["outbound-limit"]);
                        const decimals = mint.decimals;

                        const u64max = new BN(2).pow(new BN(64)).sub(new BN(1));

                        if (outboundLimit.cmp(u64max) === 1) {
                            console.error(`Outbound limit too high (max is ${u64max} tokens)`);
                            process.exit(1);
                        }

                        const adjusted = outboundLimit.div(new BN(10).pow(new BN(decimals)));

                        if (argv["mode"] === "burning" && !mint.mintAuthority?.equals(ntt.tokenAuthorityAddress())) {
                            if (mint.mintAuthority?.equals(payer.publicKey)) {
                                await askForConfirmation("Do you want to transfer mint authority to NTT program?");
                                await spl.setAuthority(
                                    connection,
                                    payer,
                                    mint.address,
                                    payer,
                                    spl.AuthorityType.MintTokens,
                                    ntt.tokenAuthorityAddress(),
                                    undefined,
                                    undefined,
                                    tokenProgram
                                );
                            } else {
                                console.error("Mint authority is not NTT program (and not signer either, so we can't transfer).");
                                process.exit(1);
                            }
                        }

                        console.log(`${mint.address.toBase58()} mint has and ${decimals} decimals`);
                        console.log(`Outbound limit is ${adjusted} tokens (adjusted)`);
                        await askForConfirmation();

                        console.log("Initializing NTT program");
                        await ntt.initialize({
                            payer,
                            owner: payer,
                            chain: "solana",
                            mint: new PublicKey(argv["mint"]),
                            outboundLimit: outboundLimit,
                            mode: argv["mode"] as "locking" | "burning",
                        });

                        console.log("Registering wormhole transceiver");
                        await ntt.registerTransceiver({
                            payer,
                            owner: payer,
                            transceiver: ntt.program.programId,
                        });

                    })
                .command("register-peer",
                    "register a peer",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("inbound-limit", {
                            alias: "i",
                            describe: "Inbound limit",
                            demandOption: true,
                            type: "string",
                        })
                        .option("chain", {
                            describe: "Chain",
                            demandOption: true,
                            type: "string",
                        })
                        .option("peer-address", {
                            describe: "Peer address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("decimals", {
                            describe: "Decimals",
                            demandOption: true,
                            type: "number",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));

                        // TODO: this only works for devnet (remove hardcode)
                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        const config = await ntt.getConfig();

                        const mint = await spl.getMint(connection, config.mint, undefined, config.tokenProgram);
                        const inboundLimit = new BN(argv["inbound-limit"]);
                        const decimals = mint.decimals;

                        const u64max = new BN(2).pow(new BN(64)).sub(new BN(1));

                        if (inboundLimit.cmp(u64max) === 1) {
                            console.error(`Outbound limit too high (max is ${u64max} tokens)`);
                            process.exit(1);
                        }

                        const adjusted = inboundLimit.div(new BN(10).pow(new BN(decimals)));
                        console.log(`${mint.address.toBase58()} mint has and ${decimals} decimals`);
                        console.log(`Inbound limit is ${adjusted} tokens (adjusted)`);
                        await askForConfirmation();

                        const chain = argv["chain"];
                        // sdkv1.assertChain(chain);

                        await ntt.setPeer({
                            payer,
                            owner: payer,
                            chain: chain as any, // TODO: version mismatch?
                            address: Buffer.from(argv["peer-address"], "hex"), // TODO: some sdk function to parse this properly
                            limit: inboundLimit,
                            tokenDecimals: argv["decimals"],
                            config
                        });
                    })
                .command("register-wormhole-transceiver-peer",
                    "register a wormhole transceiver peer",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        })
                        .option("ntt-address", {
                            alias: "a",
                            describe: "NTT address",
                            demandOption: true,
                            type: "string",
                        })
                        .option("chain", {
                            describe: "Chain",
                            demandOption: true,
                            type: "string",
                        })
                        .option("peer-address", {
                            describe: "Peer address",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        const connection = solanaConnection(argv.network);
                        const payer = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(argv["keypair"]).toString())));

                        // TODO: this only works for devnet (remove hardcode)
                        const ntt = new NTT(connection, {
                            nttId: argv["ntt-address"] as any,
                            wormholeId: "3u8hJUVTA4jH1wYAyUur7FFZVQ8H635K3tSHHF4ssjQ5", // TODO: hardcoded
                        });

                        const chain = argv["chain"];

                        await ntt.setWormholeTransceiverPeer({
                            payer,
                            owner: payer,
                            chain: chain as any, // TODO: version mismatch?
                            address: Buffer.from(argv["peer-address"], "hex"), // TODO: some sdk function to parse this properly
                        })
                    })
                .command("upgrade",
                    "upgrade the solana program",
                    (yargs) => yargs
                        .option("network", NETWORK_OPTIONS)
                        .option("dir", {
                            alias: "d",
                            describe: "Path to the solana workspace",
                            default: ".",
                            demandOption: false,
                            type: "string",
                        })
                        .option("keypair", {
                            alias: "k",
                            describe: "Path to the keypair",
                            demandOption: true,
                            type: "string",
                        }),
                    async (argv) => {
                        // TODO: the hardcoded stuff should be factored out once
                        // we support other networks and programs
                        // TODO: currently the keypair is the upgrade authority. we should support governance program too
                        const network = argv.network;
                        const keypair = argv.keypair;
                        const dir = argv.dir;
                        const objectFile = "example_native_token_transfers.so";
                        const programId = "NtTnpY76eiMYqYX9xkag2CSXk9cGi6hinMWbMLMDYUP";
                        assertNetwork(network);
                        await $`cargo build-sbf --manifest-path=${dir}/Cargo.toml --no-default-features --features "${cargoNetworkFeature(network)}"`
                        await $`solana program deploy --program-id ${programId} ${dir}/target/deploy/${objectFile} --keypair ${keypair} -u ${solanaMoniker(network)}`
                    })
                .demandCommand()
        }
    )
    .help()
    .strict()
    .demandCommand()
    .parse();

type MissingPeers<C extends Chain> = {
    managerPeers: Ntt.Peer<Chain>[];
    transceiverPeers: ChainAddress<Chain>[];
}

async function deploy<N extends Network, C extends Chain>(
    mode: Ntt.Mode,
    ch: ChainContext<N, C>,
    token: string,
    solanaPayer?: string,
    solanaProgramKeyPath?: string,
    solanaBinaryPath?: string
): Promise<ChainAddress<C>> {
    const platform = chainToPlatform(ch.chain);
    switch (platform) {
        case "Evm":
            return deployEvm(mode, ch, token);
        case "Solana":
            if (solanaPayer === undefined || !fs.existsSync(solanaPayer)) {
                console.error("Payer not found. Specify with --payer");
                process.exit(1);
            }
            return deploySolana(mode, ch, token, solanaPayer, solanaProgramKeyPath, solanaBinaryPath);
        default:
            throw new Error("Unsupported platform");
    }
    const relayer = ch.config.contracts.relayer;
    if (!relayer) {
        console.error("Relayer not found");
        process.exit(1);
    }
}

async function deployEvm<N extends Network, C extends Chain>(
    mode: Ntt.Mode,
    ch: ChainContext<N, C>,
    token: string,
): Promise<ChainAddress<C>> {
    // ensure "evm/foundry.toml" file exists.
    if (!fs.existsSync("evm/foundry.toml")) {
        console.error("Run this command from the root of an NTT project.");
        process.exit(1);
    }

    const wormhole = ch.config.contracts.coreBridge;
    if (!wormhole) {
        console.error("Core bridge not found");
        process.exit(1);
    }
    const relayer = ch.config.contracts.relayer;
    if (!relayer) {
        console.error("Relayer not found");
        process.exit(1);
    }

    const rpc = ch.config.rpc;
    const specialRelayer = "0x63BE47835c7D66c4aA5B2C688Dc6ed9771c94C74";

    // TODO: should actually make these ENV variables.
    const sig = "run(address,address,address,address,uint8)";
    const modeUint = mode === "locking" ? 0 : 1;
    const privateKey = process.env.ETH_PRIVATE_KEY;
    if (!privateKey) {
        console.error("ETH_PRIVATE_KEY is required");
        process.exit(1);
    }

    // TODO: --verify (need to take etherscan API key)
    const proc = Bun.spawn(
        ["forge",
            "script",
            "--via-ir",
            "script/DeployWormholeNtt.s.sol",
            "--rpc-url", rpc,
            "--sig", sig,
            wormhole, token, relayer, specialRelayer, modeUint.toString(),
            "--private-key", privateKey,
            "--broadcast"
        ], {
        cwd: "evm"
    });

    const out = await new Response(proc.stdout).text();

    await proc.exited;
    if (proc.exitCode !== 0) {
        process.exit(proc.exitCode ?? 1);
    }

    const logs = out.split("\n").map((l) => l.trim()).filter((l) => l.length > 0);
    const manager = logs.find((l) => l.includes("NttManager: 0x"))?.split(" ")[1];
    // const wormholeTransceiver = logs.find((l) => l.includes("WormholeTransceiver: 0x"))?.split(" ")[1];
    const universalManager = toUniversal(ch.chain, manager!);
    return { chain: ch.chain, address: universalManager };
}

async function deploySolana<N extends Network, C extends SolanaChains>(
    mode: Ntt.Mode,
    ch: ChainContext<N, C>,
    token: string,
    payer: string,
    managerKeyPath?: string,
    binaryPath?: string
): Promise<ChainAddress<C>> {
    // ensure "solana/Anchor.toml" file exists.
    if (!fs.existsSync("solana/Anchor.toml")) {
        console.error("Run this command from the root of an NTT project.");
        process.exit(1);
    }

    const wormhole = ch.config.contracts.coreBridge;
    if (!wormhole) {
        console.error("Core bridge not found");
        process.exit(1);
    }

    // grep example_native_token_transfers = ".*"
    // in solana/Anchor.toml
    // TODO: what if they rename the program?
    const existingProgramId = fs.readFileSync("solana/Anchor.toml").toString().match(/example_native_token_transfers = "(.*)"/)?.[1];
    if (!existingProgramId) {
        console.error("Program ID not found in Anchor.toml (looked for example_native_token_transfers = \"(.*)\")");
        process.exit(1);
    }

    let programKeypairPath;
    let programKeypair;

    if (managerKeyPath) {
        if (!fs.existsSync(managerKeyPath)) {
            console.error(`Program keypair not found: ${managerKeyPath}`);
            process.exit(1);
        }
        programKeypairPath = managerKeyPath;
        programKeypair = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(managerKeyPath).toString())));
    } else {
        const programKeyJson = `${existingProgramId}.json`;
        if (!fs.existsSync(programKeyJson)) {
            console.error(`Program keypair not found: ${programKeyJson}`);
            console.error("Run `solana-keygen` to create a new keypair (either with 'new', or with 'grind'), and pass it to this command with --program-key");
            process.exit(1);
        }
        programKeypairPath = programKeyJson;
        programKeypair = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(programKeyJson).toString())));
        if (existingProgramId !== programKeypair.publicKey.toBase58()) {
            console.error(`The private key in ${programKeyJson} does not match the existing program ID: ${existingProgramId}`);
            process.exit(1);
        }
    }

    // see if the program key matches the existing program ID. if not, we need
    // to update the latter in the Anchor.toml file and the lib.rs file(s)
    const providedProgramId = programKeypair.publicKey.toBase58();
    if (providedProgramId !== existingProgramId) {
        console.error(`Program keypair does not match the existing program ID: ${existingProgramId}`);
        await askForConfirmation(`Do you want to update the program ID in the Anchor.toml file and the lib.rs file to ${providedProgramId}?`);

        const anchorTomlPath = "solana/Anchor.toml";
        const libRsPath = "solana/programs/example-native-token-transfers/src/lib.rs";

        const anchorToml = fs.readFileSync(anchorTomlPath).toString();
        const newAnchorToml = anchorToml.replace(existingProgramId, providedProgramId);
        fs.writeFileSync(anchorTomlPath, newAnchorToml);
        const libRs = fs.readFileSync(libRsPath).toString();
        const newLibRs = libRs.replace(existingProgramId, providedProgramId);
        fs.writeFileSync(libRsPath, newLibRs);
    }

    let binary: string;

    const skipDeploy = false;

    if (!skipDeploy) {
        if (binaryPath) {
            binary = binaryPath;
        } else {
            // build the program
            // TODO: build with docker
            const proc = Bun.spawn(
                ["anchor",
                    "build",
                    "--", "--no-default-features", "--features", cargoNetworkFeature(ch.network)
                ], {
                cwd: "solana"
            });

            // const _out = await new Response(proc.stdout).text();

            await proc.exited;
            if (proc.exitCode !== 0) {
                process.exit(proc.exitCode ?? 1);
            }

            binary = "solana/target/deploy/example_native_token_transfers.so";
        }

        await checkSolanaBinary(binary, wormhole, providedProgramId)

        // do the actual deployment
        const deployProc = Bun.spawn(
            ["solana",
                "program",
                "deploy",
                "--program-id", programKeypairPath,
                binary,
                "--keypair", payer,
                "-u", ch.config.rpc
            ]);

        const out = await new Response(deployProc.stdout).text();

        await deployProc.exited;

        if (deployProc.exitCode !== 0) {
            process.exit(deployProc.exitCode ?? 1);
        }

        console.log(out);
    }

    // wait 3 seconds
    await new Promise((resolve) => setTimeout(resolve, 3000));

    const emitter = NTT.pdas(providedProgramId).emitterAccount().toBase58();

    const payerKeypair = Keypair.fromSecretKey(new Uint8Array(JSON.parse(fs.readFileSync(payer).toString())));

    // can't do this yet.. need to init first.
    // const {ntt, addresses} = await nttFromManager(ch, providedProgramId);
    const ntt: SolanaNtt<N, C> = await ch.getProtocol("Ntt", {
        ntt: {
            manager: providedProgramId,
            token: token,
            transceiver: { wormhole: emitter },
        }
    }) as SolanaNtt<N, C>;

    const tx = ntt.initialize(
        toUniversal(ch.chain, payerKeypair.publicKey.toBase58()),
        {
            mint: new PublicKey(token),
            mode,
            outboundLimit: 100000000n,
        });

    const signer = await getSigner(ch, encoding.b58.encode(payerKeypair.secretKey));

    try {
        await signSendWait(ch, tx, signer.signer);
    } catch (e) {
        console.error(e.logs);
    }

    return { chain: ch.chain, address: toUniversal(ch.chain, providedProgramId) };
}

async function missingPeers(
    deps: Partial<{ [C in Chain]: Deployment<C> }>,
    verbose: boolean,
): Promise<Partial<{ [C in Chain]: MissingPeers<C> }>> {
    const missingPeers: Partial<{ [C in Chain]: MissingPeers<C> }> = {};

    for (const [fromChain, from] of Object.entries(deps)) {
        assertChain(fromChain);

        for (const [toChain, to] of Object.entries(deps)) {
            assertChain(toChain);
            if (fromChain === toChain) {
                continue;
            }
            if (verbose) {
                process.stdout.write(`Verifying registration for ${fromChain} -> ${toChain}\r`);
            }
            const peer = await from.ntt.getPeer(toChain);
            if (peer === null) {
                // console.error(`Peer not found for ${fromChain} -> ${toChain}`);
                missingPeers[fromChain] = missingPeers[fromChain] || { managerPeers: [], transceiverPeers: [] };
                const configLimit = from.config.local?.limits?.inbound?.[toChain]?.replace(".", "");
                missingPeers[fromChain].managerPeers.push({
                    address: to.manager,
                    tokenDecimals: to.decimals,
                    inboundLimit: BigInt(configLimit ?? 0),
                });
            } else {
                // @ts-ignore TODO
                if (!Buffer.from(peer.address.address.address).equals(Buffer.from(to.manager.address.address))) {
                    console.error(`Peer address mismatch for ${fromChain} -> ${toChain}`);
                }
                if (peer.tokenDecimals !== to.decimals) {
                    console.error(`Peer decimals mismatch for ${fromChain} -> ${toChain}`);
                }

            }
            const transceiverPeer = await from.whTransceiver.getPeer(toChain);
            if (transceiverPeer === null) {
                // console.error(`Transceiver peer not found for ${fromChain} -> ${toChain}`);
                missingPeers[fromChain] = missingPeers[fromChain] || { managerPeers: [], transceiverPeers: [] };
                missingPeers[fromChain].transceiverPeers.push(to.whTransceiver.getAddress());
            } else {
                // @ts-ignore TODO
                if (!Buffer.from(transceiverPeer.address.address).equals(Buffer.from(to.whTransceiver.getAddress().address.address))) {
                    console.error(`Transceiver peer address mismatch for ${fromChain} -> ${toChain}`);
                }
            }

        }
    }
    return missingPeers;
}

function info(msg: string, verbose: boolean) {
    if (verbose) {
        console.info(msg);
    }
}

async function pushDeployment<C extends Chain>(deployment: Deployment<C>) {
    const diff = diffObjects(deployment.config.local!, deployment.config.remote!);
    if (Object.keys(diff).length === 0) {
        return;
    }

    const canonical = canonicalAddress(deployment.manager);
    console.log(`Pushing changes to ${deployment.manager.chain} (${canonical})`)

    console.log(chalk.reset(colorizeDiff(diff)));
    await askForConfirmation();

    const ctx = deployment.ctx;

    const signer = await getSigner(ctx)

    let txs = [];
    for (const k of Object.keys(diff)) {
        let tx;
        if (k === "paused") {
            if (diff[k]?.push === true) {
                txs.push(deployment.ntt.pause(signer.address.address));
            } else {
                txs.push(deployment.ntt.unpause(signer.address.address));
            }
        } else if (k === "limits") {
            const newOutbound = diff[k]?.outbound?.push;
            if (newOutbound) {
                // TODO: verify amount has correct number of decimals?
                // remove "." from string and convert to bigint
                const newOutboundBigint = BigInt(newOutbound.replace(".", ""));
                txs.push(deployment.ntt.setOutboundLimit(newOutboundBigint, signer.address.address));
            }
            const inbound = diff[k]?.inbound;
            if (inbound) {
                for (const chain of Object.keys(inbound)) {
                    assertChain(chain);
                    const newInbound = inbound[chain]?.push;
                    if (newInbound) {
                        // TODO: verify amount has correct number of decimals?
                        const newInboundBigint = BigInt(newInbound.replace(".", ""));
                        txs.push(deployment.ntt.setInboundLimit(chain, newInboundBigint, signer.address.address));
                    }
                }
            }
        } else {
            console.error(`Unsupported field: ${k}`);
            process.exit(1);
        }
    }
    for (const tx of txs) {
        await signSendWait(ctx, tx, signer.signer)
    }
}

async function pullDeployments(deployments: Config, network: Network, verbose: boolean): Promise<Partial<{ [C in Chain]: Deployment<C> }>> {
    let deps: Partial<{
        [C in Chain]: Deployment<C>;
    }> = {};

    for (const [chain, deployment] of Object.entries(deployments.chains)) {
        if (verbose) {
            process.stdout.write(`Fetching config for ${chain}\r`);
        }
        assertChain(chain);
        const managerAddress: string | undefined = deployment.manager;
        if (managerAddress === undefined) {
            console.error(`manager field not found for chain ${chain}`);
            // process.exit(1);
            continue;
        }
        const [remote, ctx, ntt, decimals] = await pullChainConfig(
            network,
            { chain, address: toUniversal(chain, managerAddress) },
            overrides
        );
        const local = deployments.chains[chain];

        // TODO: what if it's not index 0...
        // we should check that the address of this transceiver matches the
        // address in the config. currently we just assume that ix 0 is the wormhole one
        const whTransceiver = await ntt.getTransceiver(0);
        if (whTransceiver === null) {
            console.error(`Wormhole transceiver not found for ${chain}`);
            process.exit(1);
        }

        deps[chain] = {
            // @ts-ignore
            ctx,
            // @ts-ignore
            ntt,
            decimals,
            // @ts-ignore
            manager: { chain, address: toUniversal(chain, managerAddress) },
            // @ts-ignore
            whTransceiver,
            config: {
                remote,
                local,
            }
        };
    }

    const config = Object.fromEntries(Object.entries(deps).map(([k, v]) => [k, v.config.remote]));
    const ntts = Object.fromEntries(Object.entries(deps).map(([k, v]) => [k, v.ntt]));
    await pullInboundLimits(ntts, config, verbose);
    return deps;
}

async function pullChainConfig<N extends Network, C extends Chain>(
    network: N,
    manager: ChainAddress<C>,
    overrides?: ConfigOverrides<N>
): Promise<[ChainConfig, ChainContext<typeof network, C>, Ntt<typeof network, C>, number]> {
    const wh = new Wormhole(network, [solana.Platform, evm.Platform], overrides);
    const ch = wh.getChain(manager.chain);

    const nativeManagerAddress = canonicalAddress(manager);

    const { ntt, addresses }: { ntt: Ntt<N, C>; addresses: Partial<Ntt.Contracts>; } =
        await nttFromManager<N, C>(ch, nativeManagerAddress);

    const mode = await ntt.getMode();
    const outboundLimit = await ntt.getOutboundLimit();
    const threshold = await ntt.getThreshold();

    const decimals = await ntt.getTokenDecimals();
    // insert decimal point into number
    const outboundLimitDecimals = formatNumber(outboundLimit, decimals);

    const paused = await ntt.isPaused();
    const owner = await ntt.getOwner();

    const config: ChainConfig = {
        mode,
        paused,
        owner: owner.toString(),
        manager: nativeManagerAddress,
        token: addresses.token!,
        transceivers: {
            threshold,
            wormhole: addresses.transceiver!.wormhole!,
        },
        limits: {
            outbound: outboundLimitDecimals,
            inbound: {},
        },
    };
    return [config, ch, ntt, decimals];
}

// TODO: there should be a more elegant way to do this, than creating a
// "dummy" NTT, then calling verifyAddresses to get the contract diff, then
// finally reconstructing the "real" NTT object from that
async function nttFromManager<N extends Network, C extends Chain>(
    ch: ChainContext<N, C>,
    nativeManagerAddress: string
): Promise<{ ntt: Ntt<N, C>; addresses: Partial<Ntt.Contracts> }> {
    const onlyManager = await ch.getProtocol("Ntt", {
        ntt: {
            manager: nativeManagerAddress,
            token: null,
            transceiver: { wormhole: null },
        }
    });
    const diff = await onlyManager.verifyAddresses();

    const addresses: Partial<Ntt.Contracts> = { manager: nativeManagerAddress, ...diff };

    const ntt = await ch.getProtocol("Ntt", {
        ntt: addresses
    });
    return { ntt, addresses };
}

function formatNumber(num: bigint, decimals: number) {
    if (num === 0n) {
        return "0." + "0".repeat(decimals);
    }
    const str = num.toString();
    const formatted = str.slice(0, -decimals) + "." + str.slice(-decimals);
    if (formatted.startsWith(".")) {
        return "0" + formatted;
    }
    return formatted;
}

function cargoNetworkFeature(network: Network): string {
    switch (network) {
        case "Mainnet":
            return "mainnet";
        case "Testnet":
            return "solana-devnet";
        case "Devnet":
            return "tilt-devnet";
        default:
            throw new Error("Unsupported network");
    }
}


function solanaMoniker(network: Network): string {
    switch (network) {
        case "Mainnet":
            return "m";
        case "Testnet":
            return "d";
        case "Devnet":
            return "l";
    }
}
async function askForConfirmation(prompt: string = "Do you want to continue?"): Promise<void> {
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
    });
    const answer = await new Promise<string>((resolve) => {
        rl.question(`${prompt} [y/n]`, resolve);
    });
    rl.close();

    if (answer !== "y") {
        console.log("Aborting");
        process.exit(0);
    }
}

function solanaConnection(network: string): Connection {
    if (network === "testnet") {
        return new Connection("https://api.devnet.solana.com", "confirmed");
    } else {
        throw new Error("Unsupported network");
    }
}

// NOTE: modifies the config object in place
// TODO: maybe introduce typestate for having pulled inbound limits?
async function pullInboundLimits(ntts: Partial<{ [C in Chain]: Ntt<Network, C> }>, config: Config["chains"], verbose: boolean) {
    for (const [c1, ntt1] of Object.entries(ntts)) {
        assertChain(c1);
        const chainConf = config[c1];
        if (!chainConf) {
            console.error(`Chain ${c1} not found in deployment`);
            process.exit(1);
        }
        const decimals = await ntt1.getTokenDecimals();
        for (const [c2, ntt2] of Object.entries(ntts)) {
            assertChain(c2);
            if (ntt1 === ntt2) {
                continue;
            }
            if (verbose) {
                process.stdout.write(`Fetching inbound limit for ${c1} -> ${c2}`);
            }
            let peer: Ntt.Peer<Chain> | null = null;
            while (true) {
                try {
                    peer = await ntt1.getPeer(c2);
                    break;
                } catch (e) {
                    // @ts-ignore TODO
                    if (e.toString().includes("limit") || (JSON.stringify(e)).includes("limit")) {
                        if (verbose) {
                            process.stdout.write(` (rate limited, waiting...)`);
                        }
                        await new Promise((resolve) => setTimeout(resolve, 5000));
                        continue;
                    } else {
                        throw e;
                    }
                }
            }
            if (verbose) {
                process.stdout.write(`\r`);
            }
            if (chainConf.limits?.inbound === undefined) {
                chainConf.limits.inbound = {};
            }

            const limit = peer?.inboundLimit ?? 0n;

            chainConf.limits.inbound[c2] = formatNumber(limit, decimals)

        }
    }
}

async function checkSolanaBinary(binary: string, wormhole: string, providedProgramId: string) {
    // ensure binary path exists
    if (!fs.existsSync(binary)) {
        console.error(`.so file not found: ${binary}`);
        process.exit(1);
    }
    // console.log(`Checking binary ${binary} for wormhole and provided program ID`);

    // convert wormhole and providedProgramId from base58 to hex
    const wormholeHex = new PublicKey(wormhole).toBuffer().toString("hex");
    const providedProgramIdHex = new PublicKey(providedProgramId).toBuffer().toString("hex");

    return true;

    // TODO: these just silently fail, but work when I run them directly in the shell. why?
    await $`xxd -p ${binary} | tr -d '\\n' | grep ${wormholeHex}`;
    await $`xxd -p ${binary} | tr -d '\\n' | grep ${providedProgramIdHex}`;

}
