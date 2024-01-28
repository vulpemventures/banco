# Protocol

## How it Works

Banco uses a RFQ (Request for Quote) trading protocol and operates in two main steps: **funding** and **fulfilling** or **canceling**.

There are two actors in the protocol: the **maker** requests a covenant address from the **taker** that represents the **trade contract**.
The maker then sends a transaction to the network to fund this contract. The transaction includes the asset and quantity specified in the contract. Once the **funding transaction** is on the mempool, the taker can fulfill the contract by sending a **fulfill transaction** that spends the output of the funding transaction. This fulfill transaction must include an output that matches the requested value, asset, and script specified in the contract. If it does, the contract is considered fulfilled, and the trade is executed. If the maker decides they no longer wish to trade, they can cancel the contract. Only the maker has the ability to do this. Canceling the contract involves sending a **cancel transaction** that spends the output of the funding transaction back to the maker.

### Trade contract

The trade contract is a Taproot address that uses [Elements introspection opcodes](https://github.com/ElementsProject/elements/blob/master/doc/tapscript_opcodes.md) to constraint the unlocking conditions of the output. 

#### Spending Conditions

Unspendable key-path and two script-path branches:

- **Fulfill** branch: the spending transaction must include the first output that matches the requested value, asset, and script specified in the contract that the maker shall receive. The **taker** must add the necessary inputs to fund the enforced output and the network fees.

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

- **Cancel** branch: the spending transaction must include the first output that matches the requested value, asset, and script specified in the contract that the maker must have sent. The **maker** must add the necessary inputs to pay the network fees and eventually additional funds if the contracts was underfunded.

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

### Funding transaction

In the funding step, a participant (either the maker or taker) creates a trade contract.
This contract represents a limit order, specifying the asset, the quantity of the asset, and the address at which the maker wishes to receive the asset. The maker then sends a transaction to the network to fund this contract.

### Fulfill transaction

Once the contract is funded and broadcast to the network, any observer (the taker) can fulfill it. To do this, the taker sends a transaction that spends the output of the funding transaction. This spending transaction must include an output that matches the requested value, asset, and script specified in the contract. If it does, the contract is considered fulfilled, and the trade is executed.

### Cancel transaction

If the maker decides they no longer wish to trade, they can cancel the contract. Only the maker has the ability to do this. Canceling the contract involves sending a transaction that spends the output of the funding transaction back to the maker.
