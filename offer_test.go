package main

import (
	"testing"

	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/assert"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/psetv2"
	"github.com/vulpemventures/go-elements/taproot"
)

var traderPubKeyBytes = []byte{0x2, 0x26, 0xb7, 0xb1, 0xc5, 0xd9, 0xe6, 0xf7, 0xa1, 0x46, 0xc5, 0xa1, 0x1a, 0x3d, 0xf, 0x36, 0x5b, 0xe1, 0xc5, 0x73, 0x6d, 0xce, 0xc6, 0x12, 0xe2, 0xac, 0xeb, 0x6d, 0x45, 0xcd, 0x22, 0x29, 0x89}
var providerPubKeyBytes = []byte{0x3, 0x96, 0x2e, 0x30, 0x8e, 0xa9, 0x49, 0x34, 0x29, 0xbc, 0x14, 0xc2, 0x5a, 0x53, 0xd1, 0x2f, 0x41, 0x11, 0xf7, 0xcc, 0xe6, 0x46, 0x5c, 0xc5, 0x74, 0x7, 0x7a, 0x5c, 0x92, 0xa2, 0x97, 0x95, 0xfd}
var traderScriptExpected = []byte{0x51, 0x20, 0x54, 0x67, 0xca, 0x71, 0xd4, 0x28, 0x4c, 0x12, 0xfa, 0x73, 0xf1, 0x74, 0x67, 0x5a, 0x1a, 0xe2, 0xea, 0xc1, 0x6d, 0x1b, 0x36, 0xd0, 0xbd, 0xe6, 0x5e, 0xe3, 0x52, 0x6e, 0x3c, 0x19, 0xa9, 0x82}

func TestCreateFundingOutput(t *testing.T) {

	traderKey, err := secp256k1.ParsePubKey(traderPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	tweakedKey := taproot.ComputeTaprootKeyNoScript(traderKey)
	traderPayment, err := payment.FromTweakedKey(tweakedKey, &network.Testnet, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Assert the generated scripts
	assert.Equal(t, traderScriptExpected, traderPayment.Script)

	inputAmount := uint64(200)
	inputAsset, _ := elementsutil.AssetHashToBytes(currencyToAsset["L-BTC"].AssetHash)
	outputAmount := uint64(200)
	outputAsset, _ := elementsutil.AssetHashToBytes(currencyToAsset["USDT"].AssetHash)

	fulfillScript, _ := FulfillScript(traderPayment.Script, outputAmount, outputAsset)
	refundScript, _ := RefundScript(traderPayment.Script, inputAmount, inputAsset)

	output, err := CreateFundingOutput(fulfillScript, refundScript, &network.Testnet)
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, err)
	addr, err := output.TaprootAddress()
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, err)
	assert.Equal(t, "tex1p23k6jtxynn5jlcz6lhw03dlhyc75tw5z0ugg7fz557r0ql2w6rvq4l748y", addr)
}

func TestFulfillTransaction(t *testing.T) {
	// get a Provider key
	providerPubKey, _ := secp256k1.ParsePubKey(providerPubKeyBytes)
	providerPayment, err := payment.FromTweakedKey(taproot.ComputeTaprootKeyNoScript(providerPubKey), &network.Testnet, nil)
	if err != nil {
		t.Fatal(err)
	}
	providerAddress, _ := providerPayment.TaprootAddress()
	assert.Equal(t, "tex1pwsn99s9gn3qt3fvg8h4dn6vx3rj0r87jtj6a0g89l89ftt2c6qesgx96xr", providerAddress)

	inputArgs := []psetv2.InputArgs{
		{
			TxIndex: uint32(0),
			Txid:    "b7e6664c79fc4229504fc0521c661d431e0be2cb25be230bbaf6d8112fc89efe",
		},
		{
			TxIndex: uint32(1),
			Txid:    "b7e6664c79fc4229504fc0521c661d431e0be2cb25be230bbaf6d8112fc89efe",
		},
		{
			TxIndex: uint32(2),
			Txid:    "b7e6664c79fc4229504fc0521c661d431e0be2cb25be230bbaf6d8112fc89efe",
		},
	}

	outputArgs := []psetv2.OutputArgs{
		{
			Asset:  currencyToAsset["USDT"].AssetHash,
			Amount: 200,
			Script: traderScriptExpected,
		},
		{
			Asset:  currencyToAsset["FUSD"].AssetHash,
			Amount: 200,
			Script: providerPayment.Script,
		},
		{
			Asset:  currencyToAsset["USDT"].AssetHash,
			Amount: 300,
			Script: providerPayment.Script,
		},
		{
			Asset:  currencyToAsset["L-BTC"].AssetHash,
			Amount: 99500,
			Script: providerPayment.Script,
		},
		{
			Asset:  currencyToAsset["L-BTC"].AssetHash,
			Amount: 500,
		},
	}

	ptx, err := psetv2.New(inputArgs, outputArgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	updater, err := psetv2.NewUpdater(ptx)
	if err != nil {
		t.Fatal(err)
	}

	updater.AddInSighashType(int(txscript.SigHashDefault), 0)
	if err != nil {
		t.Fatal(err)
	}
	updater.AddInSighashType(int(txscript.SigHashDefault), 1)
	if err != nil {
		t.Fatal(err)
	}
	updater.AddInSighashType(int(txscript.SigHashDefault), 2)
	if err != nil {
		t.Fatal(err)
	}

	// Continue with the rest of the test...
}

/*
func TestTapscriptSpend(t *testing.T) {
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	blindingKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	checksigSchnorrScript, err := txscript.NewScriptBuilder().AddData(schnorr.SerializePubKey(privateKey.PubKey())).AddOp(txscript.OP_CHECKSIG).Script()
	if err != nil {
		t.Fatal(err)
	}

	tree := taproot.AssembleTaprootScriptTree(taproot.NewBaseTapElementsLeaf(checksigSchnorrScript))

	taprootPay, err := payment.FromTaprootScriptTree(privateKey.PubKey(), tree, &network.Regtest, blindingKey.PubKey())
	if err != nil {
		t.Fatal(err)
	}

	addr, err := taprootPay.ConfidentialTaprootAddress()
	if err != nil {
		t.Fatal(err)
	}

	txID, err := faucet(addr)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)

	faucetTx, err := fetchTx(txID)
	if err != nil {
		t.Fatal(err)
	}

	var utxo *transaction.TxOutput
	var vout int
	for index, out := range faucetTx.Outputs {
		if bytes.Equal(out.Script, taprootPay.Script) {
			utxo = out
			vout = index
			break
		}
	}

	if utxo == nil {
		t.Fatal("could not find utxo")
	}

	lbtc, _ := hex.DecodeString(
		"5ac9f65c0efcc4775e0baec4ec03abdde22473cd3cf33c0419ca290e0751b225",
	)
	lbtc = append([]byte{0x01}, elementsutil.ReverseBytes(lbtc)...)

	hash := faucetTx.TxHash()
	txInput := transaction.NewTxInput(hash[:], uint64(vout))

	receiverValue, _ := elementsutil.ValueToBytes(60000000)
	receiverScript, _ := hex.DecodeString("76a91439397080b51ef22c59bd7469afacffbeec0da12e88ac")
	receiverOutput := transaction.NewTxOutput(lbtc, receiverValue[:], receiverScript)

	changeValue, _ := elementsutil.ValueToBytes(39999500)
	changeOutput := transaction.NewTxOutput(lbtc, changeValue[:], taprootPay.Script) // address reuse here (change = input's script)

	feeScript := []byte{}
	feeValue, _ := elementsutil.ValueToBytes(500)
	feeOutput := transaction.NewTxOutput(lbtc, feeValue[:], feeScript)

	p, _ := pset.New([]*transaction.TxInput{txInput}, []*transaction.TxOutput{receiverOutput, changeOutput, feeOutput}, 2, 0)

	updater, err := pset.NewUpdater(p)
	if err != nil {
		t.Fatal(err)
	}

	updater.AddInSighashType(txscript.SigHashDefault, 0)
	if err != nil {
		t.Fatal(err)
	}

	updater.AddInWitnessUtxo(utxo, 0)

	blindDataLike := make([]pset.BlindingDataLike, 1)
	blindDataLike[0] = pset.PrivateBlindingKey(blindingKey.Serialize())

	outputPubKeyByIndex := make(map[int][]byte)
	outputPubKeyByIndex[0] = blindingKey.PubKey().SerializeCompressed()
	outputPubKeyByIndex[1] = blindingKey.PubKey().SerializeCompressed()

	blinder, _ := pset.NewBlinder(
		p,
		blindDataLike,
		outputPubKeyByIndex,
		nil,
		nil,
	)

	err = blinder.Blind()
	if err != nil {
		t.Fatal(err)
	}

	unsignedTx := p.UnsignedTx

	// Sign step
	genesisBlockhash, _ := chainhash.NewHashFromStr(network.Regtest.GenesisBlockHash)

	leafProof := tree.LeafMerkleProofs[0]
	leafHash := leafProof.TapHash()

	sighash := unsignedTx.HashForWitnessV1(
		0,
		[][]byte{
			utxo.Script,
		},
		[][]byte{
			utxo.Asset,
		},
		[][]byte{
			utxo.Value,
		},
		txscript.SigHashDefault,
		genesisBlockhash,
		&leafHash,
		nil,
	)

	sig, err := schnorr.Sign(privateKey, sighash[:])
	if err != nil {
		t.Fatal(err)
	}

	controlBlock := leafProof.ToControlBlock(privateKey.PubKey())
	controlBlockBytes, err := controlBlock.ToBytes()
	if err != nil {
		t.Fatal(err)
	}

	unsignedTx.Inputs[0].Witness = transaction.TxWitness{
		sig.Serialize(),
		leafProof.Script,
		controlBlockBytes,
	}

	signed, err := unsignedTx.ToHex()
	if err != nil {
		t.Fatal(err)
	}

	_, err = broadcast(signed)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, txID)
}

*/
