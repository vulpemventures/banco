# Banco: Non-Interactive Atomic Swaps

<div style="display: flex; justify-content: center;">
  <img src="non-interactive-atomic-swaps.png" alt="diagram">
</div>

## Design Goals

1. **Confidentiality**: The protocol should be confidential by default. The maker and taker should be able to trade without revealing their intentions to the public.
2. **Easieness of integration**: The protocol should be easy to use and should not require any specific wallet integration. The maker should be able to fund a trade contract with any wallet that supports sending to a Taproot address.
3. **Liquidity efficient**: The maker should be able to create a transaction that can be fulfilled by the taker at any time (as long the maker does not cancel it wih a double-spend). This allows the taker to provide liquidity via automated bots that can fulfill the transaction without the risk of the capital to be locked for a long time (as it happens with time-locked atomic-swaps).
4. **Decentralized**: The protocol should be completely decentralized and should not require any trusted third party, being non-custodial for traders and capital efficient for market makers, without the need of centralized "order-book" server or settlment facilities. The taker should simply observe the mempool for pending public contracts to fulfill.
5. **Operational Security**: The protocol should allow the taker to operate an always-on bot that can fulfill trades without the need of interacting with the maker to close the trade, avoiding hot-wallets and the need of a maker to be online and reachable to close the trade.

## How it Works

Banco uses a RFQ (Request for Quote) trading-style protocol and operates with two transactions, one to **fund** the "trade contract" and another to either **fulfill** or **cancel** it.

The **maker** funds a **trade contract** and a **taker** that fulfills it without further interaction from the maker. The contract enforces that the first output has the requested value, asset and script in the spending transaction.

1. The maker then sends a transaction to the network to fund this contract. The transaction includes the asset and quantity specified in the contract.
2. Once the **funding transaction** is in the mempool or shared privately, the taker can fulfill the contract by sending a **fulfill transaction** that spends the output of the funding transaction. This fulfill transaction must include an output that matches the value, asset, and script.
3. If the maker decides they no longer wish to trade, they can cancel the contract. Only the maker has the ability to do this. Canceling the contract involves sending a **cancel transaction** that spends the output of the funding transaction back to the maker.

## Trade contract

The trade contract is a Taproot address that uses [Elements introspection opcodes](https://github.com/ElementsProject/elements/blob/master/doc/tapscript_opcodes.md) to constraint the unlocking conditions of the output.

### Spending Conditions

#### Key Path

Maker's signature to cancel the trade, especially helpful when refunding from under-funded contracts.

#### Fulfill clause

The spending transaction must include the first output that matches the requested value, asset, and script specified in the contract that the maker shall receive. The **taker** must add the necessary inputs to fund the enforced output and the network fees.

```hack
OP_0
OP_INSPECTOUTPUTSCRIPTPUBKEY
<MAKER_WITNESS_VERSION>
OP_EQUALVERIFY
<MAKER_WITNESS_PROGRAM>
OP_EQUALVERIFY

OP_0
OP_INSPECTOUTPUTASSET
OP_DROP
<MAKER_ASSET_TO_RECEIVE>
OP_EQUALVERIFY

OP_0 
OP_INSPECTOUTPUTVALUE
OP_DROP
<MAKER_VALUE_TO_RECEIVE>
OP_EQUAL
```

#### Cancel clause

> Without a signature or hash pre-image, the script is spendable by anyone, which opens to "expensive" DoS attacks in case the contract is broadcasted publicly. The maker can mitigate this by broadcasting the contract privately to the taker. Read more in the [Known Issues](#known-issues) section.

The spending transaction must include the first output that matches the requested value, asset, and script specified in the contract that the maker must have sent. The **maker** must add the necessary inputs to pay the network fees and eventually additional funds if the contracts was underfunded.

```hack
OP_0
OP_INSPECTOUTPUTSCRIPTPUBKEY
<MAKER_WITNESS_VERSION>
OP_EQUALVERIFY
<MAKER_WITNESS_PROGRAM>
OP_EQUALVERIFY

OP_0
OP_INSPECTOUTPUTASSET
OP_DROP
<MAKER_ASSET_SENT>
OP_EQUALVERIFY

OP_0 
OP_INSPECTOUTPUTVALUE
OP_DROP
<MAKER_VALUE_SENT>
OP_EQUAL
```

## Transactions

### Funding transaction

In the funding step, a participant (either the maker or taker) creates a trade contract.
This contract represents a limit order, specifying the asset, the quantity of the asset, and the address at which the maker wishes to receive the asset. The maker then sends a transaction to the network to fund this contract.

### Fulfill transaction

Once the contract is funded and broadcast to the network, any observer (the taker) can fulfill it. To do this, the taker sends a transaction that spends the output of the funding transaction. This spending transaction must include an output that matches the requested value, asset, and script specified in the contract. If it does, the contract is considered fulfilled, and the trade is executed.

### Cancel transaction

If the maker decides they no longer wish to trade, they can cancel the contract. Only the maker has the ability to do this. Canceling the contract involves sending a transaction that spends the output of the funding transaction back to the maker.

## Considerations

- One of the main issue of interactive atomic swaps is the free-option problem. The taker can decide to not fulfill the contract and the maker is left with a locked capital or the need of a counter-party server to close the trade. This is solved by the non-interactive nature of the protocol. The maker can cancel the contract at any time, but any taker can fulfill it without the need of any interaction with the maker.
- If the maker gernerates the address and "pings" a specific taker privately, the trade is confiedential, perfect for OTC trading.
- If the maker broadcasts the address publicly (along with the taproot leaf script), the trade is public and anyonce can fulfill it. Block producers can extract financial value from transaction re-oredering, unless the blockchain supports Confidential Transactions and taker and maker can dervie a shared secret to blind the transaction.

## Known issues

### Under or Over funded contracts

Handling under-funded or over-funded contracts is not trivial.
The key-path spend it the simplest solution, but requires the maker's wallet to support the generation of the trade contracts or being able to provide a public key, to import an output script and to sign a taproot transaction. 
This goes against the goal of being easy to integrate.

You could keep an "hidden" script path to allow the maker to cancel the contract in case of under-funding.
by simply stacking fee-paying inputs to the transaction and asset inputs to matcht the original amount when under-funded.
Assuming 64 bit arithmetic opcodes are available, the maker could cancel the contract in case of under-funding calculating the actual value of the output to return on the stack.

### DoS attack

Anyone could halt a trade by spending the output via the cancel clause that returns the funds to the maker. It's reccomended to not leak the entitre taproot tree to the public, but only the fulfill leaf script.
This is not a problem for OTC trading, but it may be for public trading.
