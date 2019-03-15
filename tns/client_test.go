package tns

import (
	"context"
	"testing"

	"github.com/RTradeLtd/config"
	ci "github.com/libp2p/go-libp2p-crypto"
)

const (
	clientMaAddr = "/ip4/127.0.0.1/tcp/10050"
	daemonMaAddr = "/ip4/127.0.0.1/tcp/10051"
)

func TestClient(t *testing.T) {
	cfg, err := config.LoadConfig(testCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := NewClient(
		ctx,
		ClientOptions{true, nil, clientMaAddr},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	pk, _, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	daemon, err := NewDaemon(ctx, Options{
		ManagerPK:  pk,
		Config:     cfg,
		ListenAddr: daemonMaAddr,
		Dev:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer daemon.Close()
	go func(d *Daemon) {
		if err := d.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}(daemon)
	if _, err := client.AddPeer(daemonMaAddr + "/p2p/" + daemon.PeerID().Pretty()); err != nil {
		t.Fatal(err)
	}
}
