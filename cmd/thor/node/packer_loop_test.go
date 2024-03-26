package node

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/vechain/thor/v2/bft"
	"github.com/vechain/thor/v2/block"
	"github.com/vechain/thor/v2/chain"
	"github.com/vechain/thor/v2/comm"
	"github.com/vechain/thor/v2/genesis"
	"github.com/vechain/thor/v2/muxdb"
	"github.com/vechain/thor/v2/packer"
	"github.com/vechain/thor/v2/state"
	"github.com/vechain/thor/v2/thor"
	"github.com/vechain/thor/v2/tx"
)

func getMockFlow(t *testing.T) *packer.Flow {
	db := muxdb.NewMem()
	stater := state.NewStater(db)
	gene := genesis.NewDevnet()

	b, _, _, err := gene.Build(stater)
	if err != nil {
		t.Fatal(err)
	}
	repo, _ := chain.NewRepository(db, b)
	addr := thor.BytesToAddress([]byte("to"))
	cla := tx.NewClause(&addr).WithValue(big.NewInt(10000))
	tx := new(tx.Builder).
		ChainTag(repo.ChainTag()).
		GasPriceCoef(1).
		Expiration(10).
		Gas(21000).
		Nonce(1).
		Clause(cla).
		BlockRef(tx.NewBlockRef(0)).
		Build()

	sig, err := crypto.Sign(tx.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	tx = tx.WithSignature(sig)
	packer := packer.New(repo, stater, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address, thor.NoFork)
	sum, _ := repo.GetBlockSummary(b.Header().ID())
	flow, err := packer.Schedule(sum, uint64(time.Now().Unix()))
	if err != nil {
		t.Fatal(err)
	}

	err = flow.Adopt(tx)
	if err != nil {
		t.Fatal(err)
	}

	return flow
}

func GetNewBlock(t *testing.T) (*block.Block, tx.Receipts) {
	db := muxdb.NewMem()
	stater := state.NewStater(db)
	gene := genesis.NewDevnet()

	b, _, _, err := gene.Build(stater)
	if err != nil {
		t.Fatal(err)
	}
	repo, _ := chain.NewRepository(db, b)
	addr := thor.BytesToAddress([]byte("to"))
	cla := tx.NewClause(&addr).WithValue(big.NewInt(10000))
	transaction := new(tx.Builder).
		ChainTag(repo.ChainTag()).
		GasPriceCoef(1).
		Expiration(10).
		Gas(21000).
		Nonce(1).
		Clause(cla).
		BlockRef(tx.NewBlockRef(0)).
		Build()

	sig, err := crypto.Sign(transaction.SigningHash().Bytes(), genesis.DevAccounts()[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	transaction = transaction.WithSignature(sig)
	packer := packer.New(repo, stater, genesis.DevAccounts()[0].Address, &genesis.DevAccounts()[0].Address, thor.NoFork)
	sum, _ := repo.GetBlockSummary(b.Header().ID())
	flow, err := packer.Schedule(sum, uint64(time.Now().Unix()))
	if err != nil {
		t.Fatal(err)
	}
	err = flow.Adopt(transaction)
	if err != nil {
		t.Fatal(err)
	}
	b, stage, receipts, err := flow.Pack(genesis.DevAccounts()[0].PrivateKey, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Commit(); err != nil {
		t.Fatal(err)
	}

	return b, receipts
}

// Change genesis to have a node master be accepted as a master
// Generate key for master, derive address and set it to the node
// Make sure genesis has this master in it as one of the valid mnasters
func SetupNodeWithDependencies1(t *testing.T, ctx context.Context) (*Node, string, *comm.Communicator, *chain.Repository) {
	tempDir := SetupTempDir(t)
	db := muxdb.NewMem()
	repo := newChainRepo(db)
	pool := newPool(16, 10000)
	stater := state.NewStater(db)

	communicator := comm.NewMockCommunicator(repo, pool)
	communicator.Start()

	devAccount := genesis.DevAccounts()[0]

	master := &Master{
		PrivateKey:  devAccount.PrivateKey,
		Beneficiary: &devAccount.Address,
	}

	bftEngine, _ := bft.NewEngine(repo, db, defaultFC, thor.Address{})
	node := New(master, repo, bftEngine, stater, nil, pool, tempDir, communicator, 0, true, defaultFC)

	return node, tempDir, communicator, repo
}

func myBlockHandler(ctx context.Context, stream <-chan *block.Block) error {
	return nil
}

func TestRunPackerLoop(t *testing.T) {
	// Set a timeout for the whole test
	timeout := 20 * time.Second
	// Create a context that cancels automatically when the timeout is reached
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Ensure the context cancellation is called to free resources

	node, tempDir, comm, repo := SetupNodeWithDependencies1(t, ctx)

	// Broadcast comes after the select is run...
	go func() {
		node.packerLoop(ctx)
	}()

	time.Sleep(1 * time.Second)
	b, receipts := GetNewBlock(t)
	repo.AddBlock(b, receipts, 1)
	comm.Sync(ctx, myBlockHandler)
	repo.SetBestBlockID(b.Header().ID())

	os.Remove(tempDir)
}
