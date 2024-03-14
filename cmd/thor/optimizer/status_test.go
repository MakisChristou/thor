package optimizer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	// Update this import path to the correct path for your project
)

// Mock implementation of kv.Getter
type mockGetter struct {
	data map[string][]byte
}

func (m *mockGetter) Get(key []byte) ([]byte, error) {
	if val, ok := m.data[string(key)]; ok {
		return val, nil
	}
	return nil, errors.New("not found") // Simulate a not found error
}

func (m *mockGetter) Has(key []byte) (bool, error) {
	_, exists := m.data[string(key)]
	return exists, nil
}

func (m *mockGetter) IsNotFound(err error) bool {
	return err.Error() == "not found"
}

// Mock implementation of kv.Putter
type mockPutter struct {
	data map[string][]byte
}

func (m *mockPutter) Put(key, val []byte) error {
	m.data[string(key)] = val
	return nil
}

func (m *mockPutter) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

// Test for Load and Save
func TestStatusSaveLoad(t *testing.T) {
	mockData := make(map[string][]byte)
	getter := &mockGetter{data: mockData}
	putter := &mockPutter{data: mockData}

	originalStatus := &status{
		Base:      1,
		PruneBase: 2,
	}

	// Test Save
	err := originalStatus.Save(putter)
	assert.NoError(t, err)

	// Test Load
	loadedStatus := &status{}
	err = loadedStatus.Load(getter)
	assert.NoError(t, err)
	assert.Equal(t, originalStatus.Base, loadedStatus.Base)
	assert.Equal(t, originalStatus.PruneBase, loadedStatus.PruneBase)
}
