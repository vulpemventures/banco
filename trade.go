package main

import (
	"fmt"

	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/vulpemventures/go-elements/psetv2"
)

const FEE_AMOUNT = 500

type TradeStatus int

const (
	Pending TradeStatus = iota
	Funded
	Executed
	Cancelled
)

type Trade struct {
	Status              TradeStatus
	Order               *Order
	FundingUnspent      *UTXO
	unspentsForTrade    *[]UTXO
	unspentsForFees     *[]UTXO
	changeScriptOfTrade []byte
	changeScriptOfFees  []byte
	transaction         *psetv2.Pset
}

// FromFundedOrder accepts an Order and sets it at the funded state.
func FromFundedOrder(order *Order, fundingUnspent *UTXO) *Trade {
	// TODO does this should be raise an error instead?
	if fundingUnspent == nil {
		return FromPendingOrder(order)
	}
	// TODO check if there is a spent outpoint on the chain
	return &Trade{
		Order:          order,
		Status:         Funded,
		FundingUnspent: fundingUnspent,
	}
}

func FromPendingOrder(order *Order) *Trade {
	// TODO check if there is a spent outpoint on the chain
	return &Trade{
		Order:  order,
		Status: Pending,
	}
}

func (t *Trade) WithWalletData(
	unspentsForTrade *[]UTXO,
	unspentsForFees *[]UTXO,
	providerScriptOfTradeInput []byte,
	changeProviderScriptOfTradeOutput []byte,
	changeProviderScriptOfFees []byte,
) error {

	//inputs
	inputArgs := []psetv2.InputArgs{}
	inputArgs = append(inputArgs, psetv2.InputArgs{
		Txid:    t.FundingUnspent.Txid,
		TxIndex: uint32(t.FundingUnspent.Index),
	})

	for _, unspent := range *unspentsForTrade {
		inputArgs = append(inputArgs, psetv2.InputArgs{
			Txid:    unspent.Txid,
			TxIndex: uint32(unspent.Index),
		})
	}

	for _, unspent := range *unspentsForFees {
		inputArgs = append(inputArgs, psetv2.InputArgs{
			Txid:    unspent.Txid,
			TxIndex: uint32(unspent.Index),
		})
	}

	//outputs
	outputArgs := []psetv2.OutputArgs{
		{
			Asset:  t.Order.Output.Asset,
			Amount: t.Order.Output.Amount,
			Script: t.Order.TraderScript,
		},
		{
			Asset:  t.Order.Input.Asset,
			Amount: t.Order.Input.Amount,
			Script: providerScriptOfTradeInput,
		},
	}

	if changeProviderScriptOfTradeOutput != nil {
		// Calculate the total balance of the unspentsForTrade UTXOs
		totalBalance := uint64(0)
		for _, unspent := range *unspentsForTrade {
			totalBalance += unspent.Value
		}

		// Subtract the t.Order.Output.Amount from the total balance to check if there is a change needed
		changeAmount := totalBalance - t.Order.Output.Amount

		if changeAmount > 0 {
			outputArgs = append(outputArgs, psetv2.OutputArgs{
				Asset:  t.Order.Output.Asset,
				Amount: changeAmount,
				Script: changeProviderScriptOfTradeOutput,
			})
		}
	}

	if changeProviderScriptOfFees != nil {
		// Calculate the total balance of the unspentsForFees UTXOs
		totalBalanceFees := uint64(0)
		for _, unspent := range *unspentsForFees {
			totalBalanceFees += unspent.Value
		}

		// Subtract the fee amount from the total balance to check if there is a change needed
		changeAmountFees := totalBalanceFees - FEE_AMOUNT

		if changeAmountFees > 0 {
			outputArgs = append(outputArgs, psetv2.OutputArgs{
				Asset:  t.Order.Input.Asset,
				Amount: changeAmountFees,
				Script: changeProviderScriptOfFees,
			})
		}
	}

	outputArgs = append(outputArgs, psetv2.OutputArgs{
		Asset:  t.Order.Input.Asset,
		Amount: FEE_AMOUNT,
		Script: changeProviderScriptOfFees,
	})

	ptx, err := psetv2.New(inputArgs, outputArgs, nil)
	if err != nil {
		return fmt.Errorf("failed to create pset: %w", err)
	}
	updater, err := psetv2.NewUpdater(ptx)
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	updater.AddInSighashType(int(txscript.SigHashDefault), 0)
	if err != nil {
		return fmt.Errorf("failed to add in sighash type: %w", err)
	}
	updater.AddInSighashType(int(txscript.SigHashAll), 1)
	if err != nil {
		return fmt.Errorf("failed to add in sighash type: %w", err)
	}
	updater.AddInSighashType(int(txscript.SigHashAll), 2)
	if err != nil {
		return fmt.Errorf("failed to add in sighash type: %w", err)
	}

	// store in cache
	t.unspentsForTrade = unspentsForTrade
	t.unspentsForFees = unspentsForFees
	t.changeScriptOfTrade = changeProviderScriptOfTradeOutput
	t.changeScriptOfFees = changeProviderScriptOfFees

	return nil
}

func (t *Trade) WithSignaturesFromKey(privateKey *secp256k1.PrivateKey) {

}

func (t *Trade) ExecuteTrade() error {
	if t.Status == Pending {
		return fmt.Errorf("Trade has not being funded yet")
	}
	if t.Status == Executed || t.Status == Cancelled {
		return fmt.Errorf("Trade has already been executed or cancelled")
	}

	// Implement the logic to execute the trade here
	return nil
}

func (t *Trade) CancelTrade() error {
	// Implement the logic to cancel the trade here
	return nil
}
