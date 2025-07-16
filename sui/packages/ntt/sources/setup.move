// SPDX-License-Identifier: Apache 2

/// This module implements the mechanism to publish the NTT contract and
/// initialize `State` as a shared object.
module ntt::setup {
    use sui::coin::{TreasuryCap};

    use ntt::state;
    use ntt::mode::{Self};

    /// Capability created at `init`, which will be destroyed once
    /// `complete` is called. This ensures only the deployer can
    /// create the shared `State`.
    public struct DeployerCap has key, store {
        id: UID
    }

    /// Called automatically when module is first published. Transfers
    /// `DeployerCap` to sender.
    fun init(ctx: &mut TxContext) {
        let deployer = DeployerCap { id: object::new(ctx) };
        transfer::transfer(deployer, tx_context::sender(ctx));
    }

    #[test_only]
    public fun init_test_only(ctx: &mut TxContext) {
        init(ctx);

        // This will be created and sent to the transaction sender
        // automatically when the contract is published.
        transfer::public_transfer(
            sui::package::test_publish(object::id_from_address(@ntt), ctx),
            tx_context::sender(ctx)
        );
    }

    #[allow(lint(share_owned), lint(self_transfer))]
    /// Only the owner of the `DeployerCap` can call this method. This
    /// method destroys the capability and shares the `State` object.
    public fun complete<CoinType>(
        deployer: DeployerCap,
        upgrade_cap: sui::package::UpgradeCap,
        chain_id: u16,
        is_burning_mode: bool,  // true for burning, false for locking
        ctx: &mut TxContext
    ) {
        // Destroy deployer cap
        let DeployerCap { id } = deployer;
        object::delete(id);

        let upgrade_cap = ntt::upgrades::new_upgrade_cap(
            upgrade_cap,
            ctx
        );

        // Convert bool to Mode enum
        let mode = if (is_burning_mode) {
            mode::burning()
        } else {
            mode::locking()
        };

        // Share new state with None treasury cap for now
        // TODO: Later add proper treasury cap handling for burning mode
        let treasury_cap = std::option::none<TreasuryCap<CoinType>>();
        let (state, admin_cap) = state::new(
            chain_id,
            mode,
            treasury_cap,
            object::id(&upgrade_cap),
            ctx
        );

        transfer::public_share_object(state);

        // Transfer capabilities to transaction sender
        transfer::public_transfer(admin_cap, tx_context::sender(ctx));
        transfer::public_transfer(upgrade_cap, tx_context::sender(ctx));
    }
}
