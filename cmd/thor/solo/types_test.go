package solo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/v2/muxdb"
)

func TestTypes(t *testing.T) {

	comm := Communicator{}
	peerStats := comm.PeersStats()
	assert.Nil(t, peerStats)

	db := muxdb.NewMem()
	repo := newChainRepo(db)

	bftEngine := NewBFTEngine(repo)
	bftFinalized := bftEngine.Finalized()
	assert.NotNil(t, bftFinalized)
}
