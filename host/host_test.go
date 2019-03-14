package host_test

import (
	"context"
	"testing"

	"github.com/RTradeLtd/tns/host"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

const (
	nodeOneIP = "/ip4/127.0.0.1/tcp/10050"
	nodeTwoIP = "/ip4/127.0.0.1/tcp/10051"
)

func TestHost(t *testing.T) {
	node1PK, _, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	node2PK, _, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	node1, err := host.NewHost(context.Background(), node1PK, nodeOneIP)
	if err != nil {
		t.Fatal(err)
	}
	defer func(h *host.Host) {
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
	}(node1)
	node2, err := host.NewHost(context.Background(), node2PK, nodeTwoIP)
	if err != nil {
		t.Fatal(err)
	}
	defer func(h *host.Host) {
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
	}(node2)
	node1PID, err := peer.IDFromPublicKey(node1PK.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node2.AddPeer(nodeOneIP + "/p2p/" + node1PID.Pretty()); err != nil {
		t.Fatal(err)
	}
	if peers := node2.GetPeers(); peers.Len() <= 1 {
		t.Fatal("bad peer count returned")
	} else {
		var foundCorrectPeer bool
		for _, v := range peers {
			if v.Pretty() == node1PID.Pretty() {
				foundCorrectPeer = true
				break
			}
		}
		if !foundCorrectPeer {
			t.Fatal("failed to find corret peer")
		}
	}
	pid, err := node1.GetPeerID()
	if err != nil {
		t.Fatal(err)
	}
	if pid.Pretty() != node1PID.Pretty() {
		t.Fatal("failed to get correct peerid")
	}
}
