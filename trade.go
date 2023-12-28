package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/vulpemventures/go-elements/psetv2"
	"github.com/vulpemventures/go-elements/taproot"
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
	Status         TradeStatus
	Order          *Order
	FundingUnspent *UTXO
	walletService  WalletService
}

type CancelTransaction struct{}

// FromFundedOrder accepts an Order and sets it at the funded state.
func FromFundedOrder(walletSvc WalletService, order *Order, fundingUnspent *UTXO) *Trade {
	// TODO does this should be raise an error instead?
	if fundingUnspent == nil {
		return FromPendingOrder(walletSvc, order)
	}
	// TODO check if there is a spent outpoint on the chain
	return &Trade{
		walletService:  walletSvc,
		Order:          order,
		Status:         Funded,
		FundingUnspent: fundingUnspent,
	}
}

func FromPendingOrder(walletSvc WalletService, order *Order) *Trade {
	return &Trade{
		walletService: walletSvc,
		Order:         order,
		Status:        Pending,
	}
}

func (t *Trade) PrepareFulfillTransaction(
	unspentsForTrade *[]UTXO,
	unspentsForFees *[]UTXO,
	providerScriptOfTradeInput []byte,
	changeProviderScriptOfTradeOutput []byte,
	changeProviderScriptOfFees []byte,
	changeProviderAmountOfTradeOutput uint64,
	changeProviderAmountOfFees uint64,
) (*psetv2.Pset, error) {

	println(changeProviderAmountOfTradeOutput, changeProviderAmountOfFees)

	if t.FundingUnspent == nil || t.Status < Funded {
		return nil, fmt.Errorf("the offer address is not funded or the Trade funding data is missing")
	}

	ptx, err := psetv2.New(nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create pset: %w", err)
	}
	updater, err := psetv2.NewUpdater(ptx)
	if err != nil {
		return nil, fmt.Errorf("failed to create updater: %w", err)
	}

	inputIndex := 0
	taprootTree := t.Order.PaymentData.Taproot.ScriptTree
	internalKeyBytes := append([]byte{0x02}, t.Order.PaymentData.Taproot.XOnlyInternalKey...)
	internalKey, err := secp256k1.ParsePubKey(internalKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to ParsePubKey: %w", err)
	}

	// Offer funding input
	updater.AddInputs([]psetv2.InputArgs{{
		Txid:    t.FundingUnspent.Txid,
		TxIndex: uint32(t.FundingUnspent.Index),
	}})
	updater.AddInSighashType(inputIndex, txscript.SigHashDefault)

	// update taproot stuff
	for _, proof := range taprootTree.LeafMerkleProofs {
		// compare the fulfill script to know which leaf to pick
		if reflect.DeepEqual(proof.Script, t.Order.FulfillScript) {
			controlBlock := proof.ToControlBlock(internalKey)

			if err := updater.AddInTapLeafScript(inputIndex, psetv2.TapLeafScript{
				TapElementsLeaf: taproot.NewBaseTapElementsLeaf(proof.Script),
				ControlBlock:    controlBlock,
			}); err != nil {
				return nil, err
			}

		}
	}
	inputIndex++

	// Trading inputs
	for _, unspent := range *unspentsForTrade {
		updater.AddInputs([]psetv2.InputArgs{{
			Txid:    unspent.Txid,
			TxIndex: uint32(unspent.Index),
		}})
		updater.AddInWitnessUtxo(inputIndex, unspent.Prevout)
		updater.AddInSighashType(inputIndex, txscript.SigHashAll)
		inputIndex++
	}

	// Fee supplier inputs
	for _, unspent := range *unspentsForFees {
		updater.AddInputs([]psetv2.InputArgs{{
			Txid:    unspent.Txid,
			TxIndex: uint32(unspent.Index),
		}})
		updater.AddInWitnessUtxo(inputIndex, unspent.Prevout)
		updater.AddInSighashType(inputIndex, txscript.SigHashAll)
		inputIndex++
	}

	//outputs
	println(t.Order.Output.Amount, t.Order.Output.Asset, hex.EncodeToString(t.Order.TraderScript))
	println(t.Order.Input.Amount, t.Order.Input.Asset, hex.EncodeToString(providerScriptOfTradeInput))

	updater.AddOutputs([]psetv2.OutputArgs{
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
	})

	if changeProviderAmountOfTradeOutput > 0 {
		println("yes")
		updater.AddOutputs([]psetv2.OutputArgs{{
			Asset:  t.Order.Output.Asset,
			Amount: changeProviderAmountOfTradeOutput,
			Script: changeProviderScriptOfTradeOutput,
		}})
	}

	if changeProviderAmountOfFees > 0 {
		println("yes yes")
		updater.AddOutputs([]psetv2.OutputArgs{{
			Asset:  currencyToAsset["tL-BTC"].AssetHash,
			Amount: changeProviderAmountOfFees,
			Script: changeProviderScriptOfFees,
		}})
	}

	updater.AddOutputs([]psetv2.OutputArgs{{
		Asset:  currencyToAsset["tL-BTC"].AssetHash,
		Amount: FEE_AMOUNT,
	}})

	return ptx, nil
}

func (t *Trade) ExecuteTrade() error {
	if t.Status == Pending {
		return fmt.Errorf("trade has not being funded yet")
	}
	if t.Status == Executed || t.Status == Cancelled {
		return fmt.Errorf("trade has already been executed or cancelled")
	}

	// Get an Address to receive the Trade Input amount
	_, providerScript, err := t.walletService.GetAddress(context.Background(), false)
	if err != nil {
		return fmt.Errorf("error in GetAddress: %w", err)
	}
	_, providerChangeScript, err := t.walletService.GetAddress(context.Background(), true)
	if err != nil {
		return fmt.Errorf("error in GetAddress: %w", err)
	}
	_, feeChangeScript, err := t.walletService.GetAddress(context.Background(), true)
	if err != nil {
		return fmt.Errorf("error in GetAddress: %w", err)
	}

	// fund the Trade Output amount of the swap
	utxosForTrade, changeAmountForTrade, err := t.walletService.SelectUtxos(context.Background(), t.Order.Output.Asset, t.Order.Output.Amount)
	if err != nil {
		return fmt.Errorf("error in SelectUtxos for trade: %w", err)
	}

	// subsidize the tx fees
	utxosForFees, changeAmountForFees, err := t.walletService.SelectUtxos(context.Background(), currencyToAsset["tL-BTC"].AssetHash, FEE_AMOUNT)
	if err != nil {
		return fmt.Errorf("error in SelectUtxos for fees: %w", err)
	}
	ptx, err := t.PrepareFulfillTransaction(&utxosForTrade, &utxosForFees, providerScript, providerChangeScript, feeChangeScript, changeAmountForTrade, changeAmountForFees)
	if err != nil {
		return fmt.Errorf("error in PrepareFulfillTransaction")
	}

	pbase64, err := ptx.ToBase64()
	if err != nil {
		return fmt.Errorf("error in ToBase64")
	}

	base64, err := t.walletService.SignPset(context.Background(), pbase64, false)
	if err != nil {
		return fmt.Errorf("error in SignPset: %w", err)
	}
	tx, err := psetv2.NewPsetFromBase64(base64)
	if err != nil {
		return fmt.Errorf("error in decoding base64: %w", err)
	}

	err = psetv2.Finalize(tx, 1)
	if err != nil {
		return fmt.Errorf("error in finalize: %w", err)
	}
	err = psetv2.Finalize(tx, 2)
	if err != nil {
		return fmt.Errorf("error in finalize: %w", err)
	}

	println(tx.ToBase64())

	return nil
}

func (t *Trade) CancelTrade() error {
	// Implement the logic to cancel the trade here
	return nil
}
