package bandwidth

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/vechain/thor/v2/block"
	"github.com/vechain/thor/v2/thor"
)

func TestBandwidthValue(t *testing.T) {
	b := &Bandwidth{}
	expected := uint64(100)
	b.value = expected // set directly for testing
	if val := b.Value(); val != expected {
		t.Errorf("expected value %d, got %d", expected, val)
	}
}

func TestBandwidthUpdate(t *testing.T) {
	b := &Bandwidth{
		value: 123,
	}
	var sig [65]byte
	rand.Read(sig[:])

	block := new(block.Builder).Build().WithSignature(sig[:])
	header := block.Header()

	// First update
	value, updated := b.Update(header, time.Second)
	if !updated {
		t.Errorf("expected true for update, got %t", updated)
	}
	if value == 123 {
		t.Errorf("expected bandwidth value to update, but it did not")
	}

}

func TestBandwidthSuggestGasLimit(t *testing.T) {
	b := &Bandwidth{}
	b.value = 1000 // Example value for testing
	expectedLimit := uint64(float64(b.value) * float64(thor.TolerableBlockPackingTime) / float64(time.Second))
	if suggestedLimit := b.SuggestGasLimit(); suggestedLimit != expectedLimit {
		t.Errorf("expected suggested gas limit %d, got %d", expectedLimit, suggestedLimit)
	}
}
