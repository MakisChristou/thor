package optimizer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/v2/block"
	"github.com/vechain/thor/v2/chain"
	"github.com/vechain/thor/v2/genesis"
	"github.com/vechain/thor/v2/muxdb"
	"github.com/vechain/thor/v2/packer"
	"github.com/vechain/thor/v2/state"
	"github.com/vechain/thor/v2/thor"
)

func newBlock(parent *block.Block, repo *chain.Repository, stater *state.Stater, forkConfig thor.ForkConfig) (*block.Block, error) {
	proposer := genesis.DevAccounts()[0]
	p := packer.New(repo, stater, proposer.Address, &proposer.Address, forkConfig)
	parentSum, _ := repo.GetBlockSummary(parent.Header().ID())
	flow, err := p.Schedule(parentSum, parent.Header().Timestamp()+thor.BlockInterval)
	if err != nil {
		return nil, err
	}
	blk, stage, receipts, err := flow.Pack(proposer.PrivateKey, 0, false)
	if err != nil {
		return nil, err
	}
	if _, err := stage.Commit(); err != nil {
		return nil, err
	}
	if err := repo.AddBlock(blk, receipts, 0); err != nil {
		return nil, err
	}
	if err := repo.SetBestBlockID(blk.Header().ID()); err != nil {
		return nil, err
	}
	return blk, nil
}

func TestNewOptimizer(t *testing.T) {
	db := muxdb.NewMem()
	stater := state.NewStater(db)
	gene := genesis.NewDevnet()
	b0, _, _, err := gene.Build(stater)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := chain.NewRepository(db, b0)
	if err != nil {
		t.Fatal(err)
	}
	forkConfig := thor.ForkConfig{}
	best := b0
	// Setup 100k blocks in order to trigger the optimizer's paths
	for i := 0; i < 100000; i++ {
		blk, err := newBlock(best, repo, stater, forkConfig)
		if err != nil {
			t.Fatal(err)
		}
		best = blk
	}
	t.Log(best.Header())

	optimizer := New(db, repo, true)

	time.Sleep(10 * time.Second)
	optimizer.Stop()
	optimizer.cancel()

	assert.NotNil(t, optimizer)
}
