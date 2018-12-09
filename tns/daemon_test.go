package tns

import (
	"context"
	"testing"
	"time"

	"github.com/RTradeLtd/database"
	"github.com/RTradeLtd/kaas"
	"github.com/RTradeLtd/rtfs"
	"github.com/jinzhu/gorm"

	"github.com/RTradeLtd/config"
	ci "github.com/libp2p/go-libp2p-crypto"
)

const (
	testCfgPath = "../testenv/config.json"
)

func TestNewDaemon(t *testing.T) {
	cfg, err := config.LoadConfig(testCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	db, err := loadDatabase(cfg)
	if err != nil {
		t.Fatal(err)
	}
	kbc, err := kaas.NewClient(cfg.Endpoints)
	if err != nil {
		t.Fatal(err)
	}
	ipfs, err := rtfs.NewManager(
		cfg.IPFS.APIConnection.Host+":"+cfg.IPFS.APIConnection.Port,
		nil, time.Minute*10,
	)
	if err != nil {
		t.Fatal(err)
	}
	pk, _, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	daemon, err := NewDaemon(&DaemonOpts{
		ManagerPK: pk,
		LogFile:   "./templogs.log",
		DB:        db,
		IPFS:      ipfs,
		KBC:       kbc,
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
	return database.OpenDBConnection(database.DBOptions{
		User:           cfg.Database.Username,
		Password:       cfg.Database.Password,
		Address:        cfg.Database.URL,
		Port:           cfg.Database.Port,
		SSLModeDisable: true,
	})
}
