module ntt::state {
    use sui::coin::TreasuryCap;
    use sui::table::{Self, Table};
    use sui::balance::Balance;
    use sui::clock::Clock;
    use wormhole::bytes32::{Self, Bytes32};
    use wormhole::external_address::ExternalAddress;
    use ntt::mode::Mode;
    use ntt::peer::{Self, Peer};
    use ntt_common::bitmap::Bitmap;
    use ntt::outbox::{Self, Outbox};
    use ntt::inbox::{Self, Inbox, InboxItem};
    use ntt::transceiver_registry::{Self, TransceiverRegistry};
    use ntt_common::native_token_transfer::NativeTokenTransfer;
    use ntt_common::ntt_manager_message::{Self, NttManagerMessage};

    /// NOTE: this is a shared object, so anyone can grab a mutable reference to
    /// it. Thus, functions are access-controlled by (package) visibility.
    public struct State<phantom T> has key, store {
        id: UID,
        mode: Mode,
        /// balance of locked tokens (in burning mode, it's always empty)
        balance: Balance<T>, // TODO: rename to custody or something
        threshold: u8,
        /// treasury cap for managing wrapped asset (in locking mode, it's None)
        treasury_cap: Option<TreasuryCap<T>>,
        peers: Table<u16, Peer>,
        outbox: Outbox<NativeTokenTransfer>,
        inbox: Inbox<NativeTokenTransfer>,
        transceivers: TransceiverRegistry,
        chain_id: u16,
        next_sequence: u64,

        version: u64,

        // for off-chain discoverability
        admin_cap_id: ID,
    }

    public(package) fun new<CoinType>(
        chain_id: u16,
        mode: Mode,
        treasury_cap: Option<TreasuryCap<CoinType>>,
        ctx: &mut TxContext
    ): (State<CoinType>, AdminCap) {
        // treasury_cap is None iff we're in locking mode
        assert!(treasury_cap.is_none() == mode.is_locking());
        let admin_cap = AdminCap {
            id: object::new(ctx)
        };

        let u64max = 0xFFFFFFFFFFFFFFFF;
        let state = State {
            id: object::new(ctx),
            mode,
            treasury_cap,
            balance: sui::balance::zero(),
            threshold: 0,
            peers: table::new(ctx),
            outbox: outbox::new(u64max, ctx),
            inbox: inbox::new(ctx),
            transceivers: transceiver_registry::new(ctx),
            chain_id,
            next_sequence: 1,
            version: 0,
            admin_cap_id: admin_cap.id.to_inner(),
        };

        (state, admin_cap)
    }

    #[test_only]
    public fun mint_for_test<T>(self: &mut State<T>, amount: u64, ctx: &mut TxContext): sui::coin::Coin<T> {
        self.treasury_cap.borrow_mut().mint(amount, ctx)
    }

    public(package) fun set_version<T>(self: &mut State<T>, new_version: u64) {
        assert!(new_version >= self.version);
        self.version = new_version;
    }

    public fun get_version<T>(self: &State<T>): u64 {
        self.version
    }

    public fun borrow_mode<T>(self: &State<T>): &Mode {
        &self.mode
    }

    public fun get_chain_id<T>(self: &State<T>): u16 {
        self.chain_id
    }

    public(package) fun borrow_balance_mut<T>(self: &mut State<T>): &mut Balance<T> {
        &mut self.balance
    }

    public(package) fun borrow_balance<T>(self: &State<T>): &Balance<T> {
        &self.balance
    }

    public fun get_threshold<T>(self: &State<T>): u8 {
        self.threshold
    }

    public(package) fun borrow_treasury_cap_mut<T>(self: &mut State<T>): &mut TreasuryCap<T> {
        self.treasury_cap.borrow_mut()
    }

    public(package) fun borrow_treasury_cap<T>(self: &State<T>): &TreasuryCap<T> {
        self.treasury_cap.borrow()
    }

    public(package) fun borrow_outbox_mut<T>(self: &mut State<T>): &mut Outbox<NativeTokenTransfer> {
        &mut self.outbox
    }

    public fun borrow_outbox<T>(self: &State<T>): &Outbox<NativeTokenTransfer> {
        &self.outbox
    }

    public fun get_enabled_transceivers<T>(self: &State<T>): Bitmap {
        self.transceivers.get_enabled_transceivers()
    }

    public fun borrow_peer<T>(self: &State<T>, chain: u16): &Peer {
        self.peers.borrow(chain)
    }

    public(package) fun borrow_peer_mut<T>(
        self: &mut State<T>,
        chain: u16
    ): &mut Peer {
        self.peers.borrow_mut(chain)
    }

    public(package) fun try_release_in<T>(
        self: &mut State<T>,
        chain_id: u16,
        message: NttManagerMessage<NativeTokenTransfer>,
        clock: &Clock
    ): bool {
        let inbox_key = inbox::new_inbox_key(chain_id, message);
        self.inbox.borrow_inbox_item_mut(inbox_key).try_release(clock)
    }

    public fun borrow_inbox_item<T>(
        self: &State<T>,
        chain_id: u16,
        message: NttManagerMessage<NativeTokenTransfer>
    ): &InboxItem<NativeTokenTransfer> {
        let inbox_key = inbox::new_inbox_key(chain_id, message);
        self.inbox.borrow_inbox_item(inbox_key)
    }

    public(package) fun borrow_inbox_item_mut<T>(
        self: &mut State<T>,
        chain_id: u16,
        message: NttManagerMessage<NativeTokenTransfer>
    ): &mut InboxItem<NativeTokenTransfer> {
        let inbox_key = inbox::new_inbox_key(chain_id, message);
        self.inbox.borrow_inbox_item_mut(inbox_key)
    }

    public fun create_transceiver_message<TransceiverAuth, T>(
        self: &mut State<T>,
        message_id: Bytes32,
        clock: &Clock
    ): ntt_common::outbound_message::OutboundMessage<ntt::auth::ManagerAuth, TransceiverAuth> {
        let transceiver_index = self.transceivers.transceiver_id<TransceiverAuth>();
        let outbox_key = outbox::new_outbox_key(message_id);
        let released = self.outbox.try_release(outbox_key, transceiver_index, clock);
        assert!(released);
        let outbox_item = self.outbox.borrow(outbox_key);
        let message = *outbox_item.borrow_data();
        let recipient_ntt_manager = *outbox_item.borrow_recipient_ntt_manager_address();
        let message = ntt_manager_message::map!(message, |m| m.to_bytes());
        ntt_common::outbound_message::new(&ntt::auth::new_auth(), message, recipient_ntt_manager)
    }

    // TODO: this currently allows a disabled transceiver to vote. should we disallow that?
    // disabled votes don't count towards the threshold, so it's not a problem
    public(package) fun vote<Transceiver, T>(
        self: &mut State<T>,
        chain_id: u16,
        message: NttManagerMessage<NativeTokenTransfer>
    ) {
        let transceiver_index = self.transceivers.transceiver_id<Transceiver>();
        let inbox_key = inbox::new_inbox_key(chain_id, message);
        self.inbox.vote(transceiver_index, inbox_key)
    }

    public(package) fun next_message_id<T>(self: &mut State<T>): Bytes32 {
        let sequence = self.next_sequence;
        self.next_sequence = sequence + 1;
        bytes32::from_u256_be(sequence as u256)
    }

    ////// Admin stuff

    public struct AdminCap has key, store {
        id: UID
    }

    public fun set_peer<T>(
        _: &AdminCap,
        state: &mut State<T>,
        chain: u16,
        address: ExternalAddress,
        token_decimals: u8,
        inbound_limit: u64,
        clock: &Clock
    ) {
        if (state.peers.contains(chain)) {
            let existing_peer = state.peers.borrow_mut(chain);
            existing_peer.set_address(address);
            existing_peer.set_token_decimals(token_decimals);
            existing_peer.borrow_inbound_rate_limit_mut().set_limit(inbound_limit, clock);
        } else {
            state.peers.add(chain, peer::new(address, token_decimals, inbound_limit))
        }
    }

    public fun set_outbound_rate_limit<T>(
        _: &AdminCap,
        state: &mut State<T>,
        limit: u64,
        clock: &Clock
    ) {
        state.outbox.borrow_rate_limit_mut().set_limit(limit, clock)
    }

    public fun set_threshold<T>(
        _: &AdminCap,
        state: &mut State<T>,
        threshold: u8
    ) {
        state.threshold = threshold
    }

    public fun register_transceiver<Transceiver, T>(self: &mut State<T>, _: &AdminCap) {
        self.transceivers.register_transceiver<Transceiver>();
    }

    public fun enable_transceiver<T>(self: &mut State<T>, _: &AdminCap, id: u8) {
        self.transceivers.enable_transceiver(id);
    }

    public fun disable_transceiver<T>(self: &mut State<T>, _: &AdminCap, id: u8) {
        self.transceivers.disable_transceiver(id);
    }
}
