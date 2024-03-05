package node

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/v2/bft"
	"github.com/vechain/thor/v2/chain"
	"github.com/vechain/thor/v2/comm"
	"github.com/vechain/thor/v2/genesis"
	"github.com/vechain/thor/v2/muxdb"
	"github.com/vechain/thor/v2/state"
	"github.com/vechain/thor/v2/thor"
	"github.com/vechain/thor/v2/txpool"
)

var defaultFC = thor.ForkConfig{
	VIP191:    math.MaxUint32,
	ETH_CONST: math.MaxUint32,
	BLOCKLIST: math.MaxUint32,
	ETH_IST:   math.MaxUint32,
	VIP214:    math.MaxUint32,
	FINALITY:  0,
}

func newChainRepo(db *muxdb.MuxDB) *chain.Repository {
	gene := genesis.NewDevnet()
	b0, _, _, _ := gene.Build(state.NewStater(db))
	repo, _ := chain.NewRepository(db, b0)
	return repo
}

func newPool(limit int, limitPerAccount int) *txpool.TxPool {
	db := muxdb.NewMem()
	repo := newChainRepo(db)
	return txpool.New(repo, state.NewStater(db), txpool.Options{
		Limit:           limit,
		LimitPerAccount: limitPerAccount,
		MaxLifetime:     time.Hour,
	})
}

func SetupNodeWithDependencies() {

}

// SetupTempDir creates a temporary directory with a random name and returns its path.
func SetupTempDir(t *testing.T) string {
	// Create a temporary directory.
	tempDir, err := os.MkdirTemp("", "tx_stash_dir")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %s", err)
	}

	// Return the path to the temporary directory without adding any files.
	return tempDir
}

func TestNewNode(t *testing.T) {

	// Set a timeout for the whole test
	timeout := 20 * time.Second

	// Create a context that cancels automatically when the timeout is reached
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Ensure the context cancellation is called to free resources

	tempDir := SetupTempDir(t)

	db := muxdb.NewMem()
	repo := newChainRepo(db)
	pool := newPool(16, 10000)
	stater := state.NewStater(db)
	privateKey, _ := crypto.GenerateKey()

	communicator := comm.New(repo, pool)

	master := &Master{
		PrivateKey:  privateKey,
		Beneficiary: &thor.Address{},
	}

	bftEngine, _ := bft.NewEngine(repo, db, defaultFC, thor.Address{})
	node := New(master, repo, bftEngine, stater, nil, pool, tempDir, communicator, 1234, true, defaultFC)

	err := node.Run(ctx)
	assert.Nil(t, err)

	// Broadcast empty block
	// communicator.BroadcastBlock(&block.Block{})

	// Delete temp file
	os.Remove(tempDir)
}
