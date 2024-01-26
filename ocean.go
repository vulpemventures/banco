package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/transaction"
	pb "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MSATS_PER_BYTE = 110

type service struct {
	addr          string
	accountName   string
	conn          *grpc.ClientConn
	walletClient  pb.WalletServiceClient
	accountClient pb.AccountServiceClient
	txClient      pb.TransactionServiceClient
	notifyClient  pb.NotificationServiceClient
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
	TransactionNotifications(ctx context.Context) (<-chan *TransactionNotification, error)
	WatchScript(ctx context.Context, script string) error
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
	GetSpentStatus() *pb.UtxoStatus
	GetConfirmedStatus() *pb.UtxoStatus
	GetRedeemScript() string
}

type walletStatus struct {
	*pb.StatusResponse
}

type getIn struct {
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

func NewWalletService(addr, accountName string) (WalletService, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	walletClient := pb.NewWalletServiceClient(conn)
	accountClient := pb.NewAccountServiceClient(conn)
	txClient := pb.NewTransactionServiceClient(conn)
	notifyClient := pb.NewNotificationServiceClient(conn)
	svc := &service{
		addr:          addr,
		accountName:   accountName,
		conn:          conn,
		walletClient:  walletClient,
		accountClient: accountClient,
		txClient:      txClient,
		notifyClient:  notifyClient,
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
		if account.GetLabel() == svc.accountName {
			found = true
			break
		}
	}
	if !found {
		if _, err := accountClient.CreateAccountBIP44(ctx, &pb.CreateAccountBIP44Request{
			Label:          svc.accountName,
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
			AccountName:    s.accountName,
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
			AccountName:    s.accountName,
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

type TransactionNotification struct {
	TxId      string
	Confirmed bool
	Timestamp int64
}

func (s *service) TransactionNotifications(ctx context.Context) (<-chan *TransactionNotification, error) {
	//
	// Create a channel to receive notifications
	notifChan := make(chan *TransactionNotification)

	// Start the TransactionNotifications RPC
	notifStream, err := s.notifyClient.TransactionNotifications(
		ctx, &pb.TransactionNotificationsRequest{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start TransactionNotifications RPC: %w", err)
	}

	// Start a goroutine to receive notifications from the Notification RPC and send them to the channel
	go func() {
		for {
			resp, err := notifStream.Recv()
			if err != nil {
				// Handle error...
				break
			}

			notif := &TransactionNotification{
				TxId:      resp.GetTxid(),
				Confirmed: false,
				Timestamp: 0,
			}

			blockDetails := resp.GetBlockDetails()
			if blockDetails != nil {
				notif.Confirmed = true
				notif.Timestamp = blockDetails.GetTimestamp()
			}

			notifChan <- notif
		}
	}()

	return notifChan, nil
}

func (s *service) WatchScript(ctx context.Context, script string) error {
	// Start the Watch RPC
	_, err := s.notifyClient.WatchExternalScript(ctx, &pb.WatchExternalScriptRequest{Script: script})
	if err != nil {
		return fmt.Errorf("failed to start Watch RPC: %w", err)
	}
	return nil
}

func (s *service) SelectUtxos(
	ctx context.Context, asset string, value uint64,
) ([]UTXO, uint64, error) {
	res, err := s.txClient.SelectUtxos(ctx, &pb.SelectUtxosRequest{
		AccountName:  s.accountName,
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
		AccountName:      s.accountName,
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
