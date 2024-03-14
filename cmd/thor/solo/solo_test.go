package solo

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/v2/chain"
	"github.com/vechain/thor/v2/genesis"
	"github.com/vechain/thor/v2/logdb"
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

func SetupSoloWithDependencies(t *testing.T) *Solo {
	db := muxdb.NewMem()
	repo := newChainRepo(db)
	pool := newPool(16, 10000)
	stater := state.NewStater(db)

	solo := New(repo, stater, &logdb.LogDB{}, pool, 123455, true, true, defaultFC)

	return solo
}

func TestNewSolo(t *testing.T) {

	// Set a timeout for the whole test
	timeout := 20 * time.Second

	// Create a context that cancels automatically when the timeout is reached
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Ensure the context cancellation is called to free resources

	solo := SetupSoloWithDependencies(t)

	err := solo.Run(ctx)
	assert.Nil(t, err)
}

func TestNewSoloZeroGasLinmit(t *testing.T) {
	// Set a timeout for the whole test
	timeout := 20 * time.Second

	// Create a context that cancels automatically when the timeout is reached
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Ensure the context cancellation is called to free resources

	db := muxdb.NewMem()
	repo := newChainRepo(db)
	pool := newPool(16, 10000)
	stater := state.NewStater(db)

	solo := New(repo, stater, &logdb.LogDB{}, pool, 0, true, false, defaultFC)

	err := solo.Run(ctx)
	assert.Nil(t, err)
}
