package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tiero/banco/pkg/bufferutil"
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
	// Offer funding input
	updater.AddInputs([]psetv2.InputArgs{{
		Txid:    t.FundingUnspent.Txid,
		TxIndex: uint32(t.FundingUnspent.Index),
	}})
	updater.AddInWitnessUtxo(inputIndex, t.FundingUnspent.Prevout)
	updater.AddInSighashType(inputIndex, txscript.SigHashDefault)

	// update taproot stuff
	taprootTree := t.Order.PaymentData.Taproot.ScriptTree
	internalKeyBytes := append([]byte{0x02}, t.Order.PaymentData.Taproot.XOnlyInternalKey...)
	internalKey, err := secp256k1.ParsePubKey(internalKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to ParsePubKey: %w", err)
	}
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
		updater.AddOutputs([]psetv2.OutputArgs{{
			Asset:  t.Order.Output.Asset,
			Amount: changeProviderAmountOfTradeOutput,
			Script: changeProviderScriptOfTradeOutput,
		}})
	}

	if changeProviderAmountOfFees > FEE_AMOUNT {
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

	// Sign Ocean's inputs
	base64, err := t.walletService.SignPset(context.Background(), pbase64, false)
	if err != nil {
		return fmt.Errorf("error in SignPset: %w", err)
	}
	ptx, err = psetv2.NewPsetFromBase64(base64)
	if err != nil {
		return fmt.Errorf("error in decoding base64: %w", err)
	}
	err = psetv2.Finalize(ptx, 1)
	if err != nil {
		return fmt.Errorf("error in finalize: %w", err)
	}
	err = psetv2.Finalize(ptx, 2)
	if err != nil {
		return fmt.Errorf("error in finalize: %w", err)
	}

	// Manually setting the FinalScriptWitness into the unsigned tx
	// psetv2 finalizer does not support script without signature
	taprootTree := t.Order.PaymentData.Taproot.ScriptTree
	if err != nil {
		return fmt.Errorf("error in decoding tx hex: %w", err)
	}
	var leafIndex int
	foundLeaf := false
	for i, leafProof := range taprootTree.LeafMerkleProofs {
		if reflect.DeepEqual(leafProof.Script, t.Order.FulfillScript) {
			foundLeaf = true
			leafIndex = i
			break
		}
	}
	if !foundLeaf {
		return fmt.Errorf("tap script not found")
	}

	leafProof := taprootTree.LeafMerkleProofs[leafIndex]
	internalKeyBytes := append([]byte{0x02}, t.Order.PaymentData.Taproot.XOnlyInternalKey...)
	internalPubKey, err := btcec.ParsePubKey(internalKeyBytes)
	if err != nil {
		log.Fatalf("Failed to parse public key: %v", err)
	}
	controlBlock := leafProof.ToControlBlock(internalPubKey)
	controlBlockBytes, err := controlBlock.ToBytes()

	if err != nil {
		return fmt.Errorf("error in encoding control block: %w", err)
	}
	witness := [][]byte{
		leafProof.Script,
		controlBlockBytes,
	}
	serializer := bufferutil.NewSerializer(nil)
	if err := serializer.WriteVector(witness); err != nil {
		return err
	}
	ptx.Inputs[0].FinalScriptWitness = serializer.Bytes()

	finalTx, err := psetv2.Extract(ptx)
	if err != nil {
		return fmt.Errorf("error in extracting to tx hex: %w", err)
	}

	txHex, err := finalTx.ToHex()
	if err != nil {
		return fmt.Errorf("error in serializing tx hex: %w", err)
	}

	// Broadcast the transaction
	txid, err := t.walletService.BroadcastTransaction(context.Background(), txHex)
	if err != nil {
		println(txHex)
		return fmt.Errorf("error in broadcasting transaction: %w", err)
	}
	if len(txid) > 0 {
		t.Status = Executed
	}

	return nil
}

func (t *Trade) CancelTrade() error {
	t.Status = Cancelled
	// Implement the logic to cancel the trade here
	return nil
}
