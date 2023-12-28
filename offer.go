package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/taproot"
)

const (
	OP_INSPECTOUTPUTSCRIPTPUBKEY = 0xd1
	OP_INSPECTOUTPUTASSET        = 0xce
	OP_INSPECTOUTPUTVALUE        = 0xcf
	OP_PUSHCURRENTINPUTINDEX     = 0xcd
	UNSPENDABLE_POINT            = "0250929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0"
)

func CreateFundingOutput(fulfillScript []byte, refundScript []byte, net *network.Network) (*payment.Payment, error) {
	if net == nil {
		net = &network.Testnet
	}

	unspendableKeyBytes, err := hex.DecodeString(UNSPENDABLE_POINT)
	if err != nil {
		return nil, fmt.Errorf("error decoding unspendable key bytes: %w", err)
	}

	unspendableKey, err := secp256k1.ParsePubKey(unspendableKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing unspendable key: %w", err)
	}

	leafTaprootTree := taproot.AssembleTaprootScriptTree(
		taproot.NewBaseTapElementsLeaf(fulfillScript),
		taproot.NewBaseTapElementsLeaf(refundScript),
	)

	payment, err := payment.FromTaprootScriptTree(unspendableKey, leafTaprootTree, net, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating payment from taproot script tree: %w", err)
	}

	return payment, nil
}

func FulfillScript(recipientScript []byte, outputAmount uint64, outputAsset []byte) ([]byte, error) {
	fulfillScript, err := compileFulfillClause(0, recipientScript[:2], recipientScript[2:], outputAmount, outputAsset)
	if err != nil {
		return nil, fmt.Errorf("error building the fulfill script: %w", err)
	}
	return fulfillScript, nil
}

func RefundScript(recipientScript []byte, inputAmount uint64, inputAsset []byte) ([]byte, error) {
	refundScript, err := compileRefundClause(0, recipientScript[:2], recipientScript[2:], inputAmount, inputAsset)
	if err != nil {
		return nil, fmt.Errorf("error building the refund script: %w", err)
	}
	return refundScript, nil
}

func compileFulfillClause(outputIndex uint64, traderFulfillScriptVersion []byte, traderFulfillScriptProgram []byte, outputAmount uint64, outputAsset []byte) ([]byte, error) {

	index := scriptNum(outputIndex).Bytes()
	assetBuffer := elementsutil.ReverseBytes(outputAsset)
	amountBuffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBuffer, outputAmount)

	builder := txscript.NewScriptBuilder()

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTSCRIPTPUBKEY)
	builder.AddData(traderFulfillScriptVersion)
	builder.AddOp(txscript.OP_EQUALVERIFY)
	builder.AddData(traderFulfillScriptProgram)
	builder.AddOp(txscript.OP_EQUALVERIFY)

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTASSET)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(assetBuffer)
	builder.AddOp(txscript.OP_EQUALVERIFY)

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTVALUE)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(amountBuffer)
	builder.AddOp(txscript.OP_EQUAL)

	script, err := builder.Script()
	if err != nil {
		return nil, err
	}

	return script, nil
}

func compileRefundClause(outputIndex uint64, traderRefundScriptVersion []byte, traderRefundScriptProgram []byte, inputAmount uint64, inputAsset []byte) ([]byte, error) {
	index := scriptNum(outputIndex).Bytes()
	assetBuffer := elementsutil.ReverseBytes(inputAsset)
	amountBuffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBuffer, inputAmount)

	builder := txscript.NewScriptBuilder()

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTSCRIPTPUBKEY)
	builder.AddData(traderRefundScriptVersion)
	builder.AddOp(txscript.OP_EQUALVERIFY)
	builder.AddData(traderRefundScriptProgram)
	builder.AddOp(txscript.OP_EQUALVERIFY)

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTASSET)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(assetBuffer)
	builder.AddOp(txscript.OP_EQUALVERIFY)

	builder.AddData(index)
	builder.AddOp(OP_INSPECTOUTPUTVALUE)
	builder.AddOp(txscript.OP_DROP)
	builder.AddData(amountBuffer)
	builder.AddOp(txscript.OP_EQUAL)

	script, err := builder.Script()
	if err != nil {
		return nil, err
	}

	return script, nil
}
