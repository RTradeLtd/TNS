package tns

import (
	"context"
	"testing"
	"time"

	"github.com/RTradeLtd/database"
	"github.com/RTradeLtd/gorm"

	"github.com/RTradeLtd/config"
	ci "github.com/libp2p/go-libp2p-crypto"
)

const (
	testCfgPath = "../testenv/config.json"
	listenAddr  = "/ip4/127.0.0.1/tcp/10050"
)

func TestNewDaemon(t *testing.T) {
	cfg, err := config.LoadConfig(testCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	pk, _, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	daemon, err := NewDaemon(context.Background(), Options{
		ManagerPK:  pk,
		Config:     cfg,
		ListenAddr: listenAddr,
		Dev:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	if err := daemon.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func loadDatabase(cfg *config.TemporalConfig) (*gorm.DB, error) {
	dbm, err := database.New(cfg, database.Options{
		SSLModeDisable: true,
	})
	if err != nil {
		return nil, err
	}
	return dbm.DB, nil
}
