package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/transaction"
	oceanv1 "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1"
	pb "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const ACCOUNT_LABEL = "default"
const MSATS_PER_BYTE = 110

type service struct {
	addr          string
	conn          *grpc.ClientConn
	walletClient  pb.WalletServiceClient
	accountClient pb.AccountServiceClient
	txClient      pb.TransactionServiceClient
}

type WalletService interface {
	Status(ctx context.Context) (WalletStatus, error)
	GetAddress(ctx context.Context, isChange bool) (string, []byte, error)
	SelectUtxos(ctx context.Context, asset string, amount uint64) ([]UTXO, uint64, error)
	SignPset(
		ctx context.Context, pset string, extractRawTx bool,
	) (string, error)
	Transfer(ctx context.Context, outs []TxOutput) (string, error)
	BroadcastTransaction(ctx context.Context, txHex string) (string, error)
	Close()
}

type WalletStatus interface {
	IsInitialized() bool
	IsUnlocked() bool
	IsSynced() bool
}

type TxInput interface {
	GetTxid() string
	GetIndex() uint32
	GetScript() string
	GetScriptSigSize() int
	GetWitnessSize() int
}

type TxOutput interface {
	GetAmount() uint64
	GetAsset() string
	GetScript() string
}
type Utxo interface {
	GetTxid() string
	GetIndex() uint32
	GetAsset() string
	GetValue() uint64
	GetScript() string
	GetAssetBlinder() string
	GetValueBlinder() string
	GetAccountName() string
	GetSpentStatus() *oceanv1.UtxoStatus
	GetConfirmedStatus() *oceanv1.UtxoStatus
	GetRedeemScript() string
}

type walletStatus struct {
	*pb.StatusResponse
}

func (w walletStatus) IsInitialized() bool {
	return w.StatusResponse.GetInitialized()
}
func (w walletStatus) IsUnlocked() bool {
	return w.StatusResponse.GetUnlocked()
}
func (w walletStatus) IsSynced() bool {
	return w.StatusResponse.GetSynced()
}

type UTXO struct {
	Txid    string `json:"txid"`
	Index   int    `json:"index"`
	Value   uint64 `json:"value"`
	Prevout *transaction.TxOutput
	Status  struct {
		Confirmed bool `json:"confirmed"`
	} `json:"status"`
}

func NewWalletService(addr string) (WalletService, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	walletClient := pb.NewWalletServiceClient(conn)
	accountClient := pb.NewAccountServiceClient(conn)
	txClient := pb.NewTransactionServiceClient(conn)
	svc := &service{
		addr:          addr,
		conn:          conn,
		walletClient:  walletClient,
		accountClient: accountClient,
		txClient:      txClient,
	}

	ctx := context.Background()
	status, err := svc.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !(status.IsInitialized() && status.IsUnlocked()) {
		return nil, fmt.Errorf("wallet must be already initialized and unlocked")
	}

	// Create ark account at startup if needed.
	info, err := walletClient.GetInfo(ctx, &pb.GetInfoRequest{})
	if err != nil {
		return nil, err
	}
	found := false
	for _, account := range info.GetAccounts() {
		if account.GetLabel() == ACCOUNT_LABEL {
			found = true
			break
		}
	}
	if !found {
		if _, err := accountClient.CreateAccountBIP44(ctx, &pb.CreateAccountBIP44Request{
			Label:          ACCOUNT_LABEL,
			Unconfidential: true,
		}); err != nil {
			return nil, err
		}
	}

	return svc, nil
}

func (s *service) Close() {
	s.conn.Close()
}

func (s *service) Status(
	ctx context.Context,
) (WalletStatus, error) {
	res, err := s.walletClient.Status(ctx, &pb.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return walletStatus{res}, nil
}

func (s *service) GetAddress(
	ctx context.Context, isChange bool,
) (string, []byte, error) {

	var addr string
	if isChange {
		res, err := s.accountClient.DeriveChangeAddresses(ctx, &pb.DeriveChangeAddressesRequest{
			AccountName:    ACCOUNT_LABEL,
			NumOfAddresses: uint64(1),
		})
		if err != nil {
			return "", nil, err
		}
		if len(res.GetAddresses()) == 0 {
			return "", nil, err
		}
		addr = res.GetAddresses()[0]
	} else {
		res, err := s.accountClient.DeriveAddresses(ctx, &pb.DeriveAddressesRequest{
			AccountName:    ACCOUNT_LABEL,
			NumOfAddresses: uint64(1),
		})
		if err != nil {
			return "", nil, err
		}
		if len(res.GetAddresses()) == 0 {
			return "", nil, err
		}
		addr = res.GetAddresses()[0]
	}

	script, err := address.ToOutputScript(addr)
	if err != nil {
		return "", nil, err
	}

	return addr, script, nil
}

func (s *service) SelectUtxos(
	ctx context.Context, asset string, value uint64,
) ([]UTXO, uint64, error) {
	res, err := s.txClient.SelectUtxos(ctx, &pb.SelectUtxosRequest{
		AccountName:  ACCOUNT_LABEL,
		TargetAsset:  asset,
		TargetAmount: value,
	})
	if err != nil {
		return nil, 0, err
	}
	utxos := make([]UTXO, len(res.GetUtxos()))
	for i, utxo := range res.GetUtxos() {
		assetBytes, _ := elementsutil.AssetHashToBytes(utxo.Asset)
		valueBytes, _ := elementsutil.ValueToBytes(utxo.Value)
		scritpBytes, _ := hex.DecodeString(utxo.Script)
		prevout := transaction.NewTxOutput(assetBytes, valueBytes, scritpBytes)
		utxos[i] = UTXO{
			Txid:    utxo.GetTxid(),
			Index:   int(utxo.GetIndex()),
			Value:   utxo.GetValue(),
			Prevout: prevout,
			Status: struct {
				Confirmed bool `json:"confirmed"`
			}{
				Confirmed: true,
			},
		}
	}
	return utxos, res.GetChange(), nil
}

func (s *service) SignPset(
	ctx context.Context, pset string, extractRawTx bool,
) (string, error) {
	res, err := s.txClient.SignPset(ctx, &pb.SignPsetRequest{
		Pset: pset,
	})
	if err != nil {
		return "", err
	}
	signedPset := res.GetPset()
	return signedPset, nil
}

func (s *service) Transfer(
	ctx context.Context, outs []TxOutput,
) (string, error) {
	res, err := s.txClient.Transfer(ctx, &pb.TransferRequest{
		AccountName:      ACCOUNT_LABEL,
		Receivers:        outputList(outs).toProto(),
		MillisatsPerByte: MSATS_PER_BYTE,
	})
	if err != nil {
		return "", err
	}
	return res.GetTxHex(), nil
}

func (s *service) BroadcastTransaction(
	ctx context.Context, txHex string,
) (string, error) {
	res, err := s.txClient.BroadcastTransaction(
		ctx, &pb.BroadcastTransactionRequest{
			TxHex: txHex,
		},
	)
	if err != nil {
		return "", err
	}
	return res.GetTxid(), nil
}

type outputList []TxOutput

func (l outputList) toProto() []*pb.Output {
	list := make([]*pb.Output, 0, len(l))
	for _, out := range l {
		list = append(list, &pb.Output{
			Amount: out.GetAmount(),
			Script: out.GetScript(),
			Asset:  out.GetAsset(),
		})
	}
	return list
}
