package p2pprotocol

import (
	"math/big"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/downloader"
)

//import "github.com/ethereum/go-ethereum/eth/gasprice"

// SWCAPIBackend implements ethapi.Backend for full nodes
type SWCAPIBackend struct {
	swc *SwarmChain
	//gpo *gasprice.Oracle
}


func (b *SWCAPIBackend) ChainConfig() *params.ChainConfig {
	return b.swc.chainConfig
}

func (b *SWCAPIBackend) CurrentBlock() *types.Block {
	return b.swc.blockchain.CurrentBlock()
}

func (b *SWCAPIBackend) SetHead(number uint64) {
	b.swc.protocolManager.downloader.Cancel()
	b.swc.blockchain.SetHead(number)
}

func (b *SWCAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.swc.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.swc.blockchain.CurrentBlock().Header(), nil
	}
	return b.swc.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *SWCAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.swc.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.swc.blockchain.CurrentBlock(), nil
	}
	return b.swc.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *SWCAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.swc.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.swc.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *SWCAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.swc.blockchain.GetBlockByHash(hash), nil
}

func (b *SWCAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	if number := rawdb.ReadHeaderNumber(b.swc.chainDb, hash); number != nil {
		return rawdb.ReadReceipts(b.swc.chainDb, hash, *number), nil
	}
	return nil, nil
}

func (b *SWCAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	number := rawdb.ReadHeaderNumber(b.swc.chainDb, hash)
	if number == nil {
		return nil, nil
	}
	receipts := rawdb.ReadReceipts(b.swc.chainDb, hash, *number)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *SWCAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.swc.blockchain.GetTdByHash(blockHash)
}

func (b *SWCAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.swc.BlockChain(), nil)
	return vm.NewEVM(context, state, b.swc.chainConfig, vmCfg), vmError, nil
}

func (b *SWCAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.swc.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *SWCAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.swc.BlockChain().SubscribeChainEvent(ch)
}

func (b *SWCAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.swc.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *SWCAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.swc.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *SWCAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.swc.BlockChain().SubscribeLogsEvent(ch)
}

func (b *SWCAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.swc.txPool.AddLocal(signedTx)
}

func (b *SWCAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.swc.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *SWCAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.swc.txPool.Get(hash)
}

func (b *SWCAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.swc.txPool.State().GetNonce(addr), nil
}

func (b *SWCAPIBackend) Stats() (pending int, queued int) {
	return b.swc.txPool.Stats()
}

func (b *SWCAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.swc.TxPool().Content()
}

func (b *SWCAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.swc.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *SWCAPIBackend) Downloader() *downloader.Downloader {
	return b.swc.Downloader()
}

func (b *SWCAPIBackend) ProtocolVersion() int {
	return b.swc.swcVersion()
}

func (b *SWCAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *SWCAPIBackend) ChainDb() ethdb.Database {
	return b.swc.ChainDb()
}

func (b *SWCAPIBackend) EventMux() *event.TypeMux {
	return b.swc.EventMux()
}