package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/v2/api/doc"
	"github.com/vechain/thor/v2/genesis"
	"github.com/vechain/thor/v2/state"
	"github.com/vechain/thor/v2/thor"
	"github.com/vechain/thor/v2/txpool"
	"gopkg.in/urfave/cli.v1"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLoadOrGeneratePrivateKey(t *testing.T) {
	filePath := "/tmp/temp.key"
	if _, err := os.Stat(filePath); err == nil {
		_ = os.Remove(filePath)
	}

	newKey, err := loadOrGeneratePrivateKey(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, newKey)

	loadedKey, err := loadOrGeneratePrivateKey(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, loadedKey)
}

func Test_defaultConfigDir(t *testing.T) {
	assert.Equal(t, filepath.Join(homeDir(), ".org.vechain.thor"), defaultConfigDir())
}

func Test_defaultDataDir(t *testing.T) {
	home := homeDir()
	expectedPath := ""

	switch runtime.GOOS {
	case "darwin":
		expectedPath = filepath.Join(home, "Library", "Application Support", "org.vechain.thor")
	case "windows":
		expectedPath = filepath.Join(home, "AppData", "Roaming", "org.vechain.thor")
	default:
		expectedPath = filepath.Join(home, ".org.vechain.thor")
	}

	result := defaultDataDir()
	assert.Equal(t, expectedPath, result)
}

func Test_handleExitSignal(t *testing.T) {
	ctx := handleExitSignal()

	// Simulate sending a SIGTERM signal
	go func() {
		time.Sleep(100 * time.Millisecond) // Wait a bit to ensure the signal handling goroutine is listening
		err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		assert.NoError(t, err)
	}()

	// Wait for the context to be cancelled or timeout
	select {
	case <-ctx.Done():
		// Context was cancelled, test should pass
	case <-time.After(1 * time.Second):
		t.Fatal("Context was not cancelled after receiving exit signal")
	}
}

func Test_requestBodyLimit(t *testing.T) {
	// A simple handler that reads the body and writes it back in the response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the body (not doing anything with it here, just for demonstration)
		_, err := io.ReadAll(r.Body)
		if err != nil {
			// If there's an error reading the body, respond with an appropriate error code
			http.Error(w, "Error reading body", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the testHandler with requestBodyLimit
	limitedHandler := requestBodyLimit(testHandler)

	// Create a test server using the limited handler
	server := httptest.NewServer(limitedHandler)
	defer server.Close()

	type testCase struct {
		description    string
		bodySize       int
		expectedStatus int
	}

	tests := []testCase{
		{"Request with body within limit", 100 * 1024, http.StatusOK},                       // 100 KB
		{"Request with body size equal to limit", 200 * 1024, http.StatusOK},                // 200 KB
		{"Request with body exceeding limit", 300 * 1024, http.StatusRequestEntityTooLarge}, // 300 KB
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			body := bytes.Repeat([]byte("a"), tc.bodySize)
			resp, err := http.Post(server.URL, "text/plain", bytes.NewReader(body))

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			_ = resp.Body.Close()
		})
	}
}

func Test_handleXGenesisID(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a test genesis ID
	genesisID := thor.Bytes32{}
	copy(genesisID[:], hexutil.MustDecode("0x"+strings.Repeat("1", 64))) // Just an example genesis ID

	// Wrap the mock handler with handleXGenesisID
	handler := handleXGenesisID(mockHandler, genesisID)

	tests := []struct {
		name           string
		headerValue    string
		queryValue     string
		expectedStatus int
	}{
		{
			name:           "Correct ID in Header",
			headerValue:    genesisID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Correct ID in Query",
			queryValue:     genesisID.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Incorrect ID in Header",
			headerValue:    "wrongvalue",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No ID Provided",
			expectedStatus: http.StatusOK, // Assuming it's okay to not provide an ID
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tc.headerValue != "" {
				req.Header.Set("x-genesis-id", tc.headerValue)
			}
			if tc.queryValue != "" {
				q := url.Values{}
				q.Add("x-genesis-id", tc.queryValue)
				req.URL.RawQuery = q.Encode()
			}

			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code, "Expected status code does not match")
		})
	}
}

func Test_handleXThorestVersion(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := handleXThorestVersion(mockHandler)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	versionHeader := rr.Header().Get("x-thorest-ver")
	expectedVersion := doc.Version()

	assert.Equal(t, expectedVersion, versionHeader, "x-thorest-ver header should match the doc.Version()")
}

func Test_handleAPITimeout(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		// Context is cancelled, set a non-200 status code and return
		http.Error(w, "Context cancelled", http.StatusServiceUnavailable)
		return
	})

	timeout := 100 * time.Millisecond
	handler := handleAPITimeout(slowHandler, timeout)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusOK, rr.Code, "Expected the handler not to return 200 OK due to the timeout")
}

func TestSelectGenesis(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(networkFlag.Name, "", "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("No network flag", func(t *testing.T) {
		_ = set.Set(networkFlag.Name, "")

		_, _, err := selectGenesis(ctx)
		assert.Error(t, err)
	})

	t.Run("Test network", func(t *testing.T) {
		_ = set.Set(networkFlag.Name, "test")

		gene, forkConfig, err := selectGenesis(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, gene)
		assert.Equal(t, thor.GetForkConfig(gene.ID()), forkConfig)
	})

	t.Run("Main network", func(t *testing.T) {
		_ = set.Set(networkFlag.Name, "main")

		gene, forkConfig, err := selectGenesis(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, gene)
		assert.Equal(t, thor.GetForkConfig(gene.ID()), forkConfig)
	})

	t.Run("Valid custom network file", func(t *testing.T) {
		tempFilePath := "../../genesis/example.json"
		_ = set.Set(networkFlag.Name, tempFilePath)

		gene, _, err := selectGenesis(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, gene)
	})

	t.Run("Invalid file path", func(t *testing.T) {
		_ = set.Set(networkFlag.Name, "/invalid/path")

		_, _, err := selectGenesis(ctx)
		assert.Error(t, err)
	})

	t.Run("Invalid JSON content", func(t *testing.T) {
		file, err := os.CreateTemp("", "invalid-genesis-*.json")
		assert.NoError(t, err)

		_, err = file.WriteString("{invalidJson:}")
		assert.NoError(t, err)
		err = file.Close()
		assert.NoError(t, err)

		defer os.Remove(file.Name())

		_ = set.Set(networkFlag.Name, file.Name())

		_, _, err = selectGenesis(ctx)
		assert.Error(t, err)
	})
}

func Test_makeConfigDir(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(configDirFlag.Name, "", "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("No config dir specified", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, "")

		dir, err := makeConfigDir(ctx)
		assert.Error(t, err)
		assert.Equal(t, "", dir)
	})

	t.Run("Config dir specified and created", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = set.Set(configDirFlag.Name, tempDir)

		dir, err := makeConfigDir(ctx)
		assert.NoError(t, err)
		assert.Equal(t, tempDir, dir)
	})

	t.Run("Config dir specified but cannot be created", func(t *testing.T) {
		// Attempt to create a directory in a location where permission should be denied
		invalidDirPath := "/root/invalidConfigDir"

		_ = set.Set(configDirFlag.Name, invalidDirPath)

		dir, err := makeConfigDir(ctx)
		assert.Error(t, err)
		assert.Equal(t, "", dir)
	})
}

func Test_makeInstanceDir(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(dataDirFlag.Name, "", "doc")
	set.Bool(disablePrunerFlag.Name, false, "doc")
	ctx := cli.NewContext(app, set, nil)

	gene := genesis.NewTestnet()

	t.Run("No data dir specified", func(t *testing.T) {
		_ = set.Set(dataDirFlag.Name, "")

		dir, err := makeInstanceDir(ctx, gene)
		assert.Error(t, err)
		assert.Equal(t, "", dir)
	})

	t.Run("Data dir specified without pruner disabled", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = set.Set(dataDirFlag.Name, tempDir)

		dir, err := makeInstanceDir(ctx, gene)
		assert.NoError(t, err)
		expectedDir := filepath.Join(tempDir, fmt.Sprintf("instance-%x-v3", gene.ID().Bytes()[24:]))
		assert.Equal(t, expectedDir, dir)
	})

	t.Run("Data dir specified with pruner disabled", func(t *testing.T) {
		tempDir := t.TempDir()

		_ = set.Set(dataDirFlag.Name, tempDir)
		_ = set.Set(disablePrunerFlag.Name, "true")

		dir, err := makeInstanceDir(ctx, gene)
		assert.NoError(t, err)
		expectedDir := filepath.Join(tempDir, fmt.Sprintf("instance-%x-v3-full", gene.ID().Bytes()[24:]))
		assert.Equal(t, expectedDir, dir)
	})
}

func Test_openMainDB(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.Int(cacheFlag.Name, 128, "doc")
	set.Bool(disablePrunerFlag.Name, false, "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("Open database successfully", func(t *testing.T) {
		tempDir := t.TempDir()

		db, err := openMainDB(ctx, tempDir)
		assert.NoError(t, err)
		assert.NotNil(t, db)
	})

	t.Run("Fail to open database with invalid directory", func(t *testing.T) {
		invalidDir := filepath.Join(string(filepath.Separator), "invalid", "path")

		db, err := openMainDB(ctx, invalidDir)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func Test_openLogDB(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	ctx := cli.NewContext(app, set, nil)

	t.Run("Open log database successfully", func(t *testing.T) {
		tempDir := t.TempDir()

		db, err := openLogDB(ctx, tempDir)
		assert.NoError(t, err)
		assert.NotNil(t, db)
	})

	t.Run("Fail to open log database with invalid directory", func(t *testing.T) {
		invalidDir := filepath.Join(string(filepath.Separator), "invalid", "path")

		db, err := openLogDB(ctx, invalidDir)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func Test_initChainRepository(t *testing.T) {
	gene := genesis.NewTestnet()
	mainDB := openMemMainDB()
	logDB := openMemLogDB()

	repo, err := initChainRepository(gene, mainDB, logDB)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
}

func TestBeneficiary(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(beneficiaryFlag.Name, "", "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("No beneficiary flag provided", func(t *testing.T) {
		addr, err := beneficiary(ctx)
		assert.NoError(t, err)
		assert.Nil(t, addr)
	})

	t.Run("Valid beneficiary address provided", func(t *testing.T) {
		_ = set.Set(beneficiaryFlag.Name, "0x9a55A6669B2D8eE006e87085584e87ef5EaCd0F8")

		addr, err := beneficiary(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, addr)
	})

	t.Run("Invalid beneficiary address provided", func(t *testing.T) {
		_ = set.Set(beneficiaryFlag.Name, "invalid_address")

		addr, err := beneficiary(ctx)
		assert.Error(t, err)
		assert.Nil(t, addr)
	})
}

func TestMasterKeyPath(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(configDirFlag.Name, "", "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("Valid config directory", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = set.Set(configDirFlag.Name, tempDir)

		expectedPath := filepath.Join(tempDir, "master.key")
		path, err := masterKeyPath(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("Invalid config directory", func(t *testing.T) {
		// Set an invalid directory to trigger an error in makeConfigDir.
		_ = set.Set(configDirFlag.Name, "/invalid/path/to/config")

		path, err := masterKeyPath(ctx)
		assert.Error(t, err)
		assert.Empty(t, path)
	})
}

func Test_loadNodeMaster(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(configDirFlag.Name, "", "doc")
	set.String(beneficiaryFlag.Name, "", "doc")
	ctx := cli.NewContext(app, set, nil)

	t.Run("Error from masterKeyPath", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, "/invalid/path")

		master, err := loadNodeMaster(ctx)
		assert.Error(t, err)
		assert.Nil(t, master)
	})

	t.Run("Error from beneficiary", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = set.Set(configDirFlag.Name, tempDir)
		_ = set.Set(beneficiaryFlag.Name, "error")

		master, err := loadNodeMaster(ctx)
		assert.Error(t, err)
		assert.Nil(t, master)
	})

	t.Run("Successful loadNodeMaster", func(t *testing.T) {
		tempDir := t.TempDir()
		_ = set.Set(configDirFlag.Name, tempDir)
		_ = set.Set(beneficiaryFlag.Name, "0x9a55A6669B2D8eE006e87085584e87ef5EaCd0F8") // Assume valid beneficiary

		master, err := loadNodeMaster(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, master)
	})
}

func Test_newP2PComm(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(configDirFlag.Name, "", "doc")
	set.String(natFlag.Name, "any", "doc")
	set.Int(maxPeersFlag.Name, 25, "doc")
	set.Int(p2pPortFlag.Name, 30303, "doc")
	ctx := cli.NewContext(app, set, nil)

	gene := genesis.NewTestnet()
	mainDB := openMemMainDB()
	logDB := openMemLogDB()

	repo, _ := initChainRepository(gene, mainDB, logDB)

	txpoolOpt := defaultTxPoolOptions
	txPool := txpool.New(repo, state.NewStater(mainDB), txpoolOpt)

	t.Run("Error from makeConfigDir", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, "/invalid/path")

		p2pComm, err := newP2PComm(ctx, repo, txPool, "/instance/dir")
		assert.Error(t, err)
		assert.Nil(t, p2pComm)
	})

	t.Run("Error from nat.Parse", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, t.TempDir())
		_ = set.Set(natFlag.Name, "invalid_nat_value")

		p2pComm, err := newP2PComm(ctx, repo, txPool, t.TempDir())
		assert.Error(t, err)
		assert.Nil(t, p2pComm)
	})

	t.Run("Invalid peers cache", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, t.TempDir())
		_ = set.Set(natFlag.Name, "")
		instanceDir := t.TempDir()
		peersCachePath := filepath.Join(instanceDir, "peers.cache")

		// Create an invalid peers cache file
		err := os.WriteFile(peersCachePath, []byte("invalid data"), 0600)
		assert.NoError(t, err)

		p2pComm, err := newP2PComm(ctx, repo, txPool, instanceDir)
		assert.NoError(t, err) // The function should handle invalid cache gracefully
		assert.NotNil(t, p2pComm)
	})

	t.Run("Peers cache does not exist", func(t *testing.T) {
		_ = set.Set(configDirFlag.Name, t.TempDir())
		instanceDir := t.TempDir() // No need to create a peers cache file

		p2pComm, err := newP2PComm(ctx, repo, txPool, instanceDir)
		assert.NoError(t, err)
		assert.NotNil(t, p2pComm)
	})
}

func TestP2pComm_Start(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", 0)
	set.String(configDirFlag.Name, "", "doc")
	set.String(natFlag.Name, "any", "doc")
	set.Int(maxPeersFlag.Name, 25, "doc")
	set.Int(p2pPortFlag.Name, 30303, "doc")
	ctx := cli.NewContext(app, set, nil)

	gene := genesis.NewTestnet()
	mainDB := openMemMainDB()
	logDB := openMemLogDB()

	repo, _ := initChainRepository(gene, mainDB, logDB)

	txpoolOpt := defaultTxPoolOptions
	txPool := txpool.New(repo, state.NewStater(mainDB), txpoolOpt)

	_ = set.Set(configDirFlag.Name, t.TempDir())
	p2pComm, _ := newP2PComm(ctx, repo, txPool, t.TempDir())

	err := p2pComm.Start()
	defer p2pComm.Stop()

	assert.NoError(t, err)
}
