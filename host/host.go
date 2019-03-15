package host

import (
	"context"

	libp2p "github.com/libp2p/go-libp2p"
	ci "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// Host is a libp2p host
type Host struct {
	host.Host
	// pk is the nodes private key
	pk  ci.PrivKey
	ctx context.Context
}

// NewHost used to create a new libp2p host
func NewHost(ctx context.Context, identity ci.PrivKey, listenAddr string) (*Host, error) {
	lHost, err := libp2p.New(ctx,
		libp2p.Identity(identity),
		libp2p.ListenAddrStrings(listenAddr),
	)
	if err != nil {
		return nil, err
	}
	return &Host{
		lHost,
		identity,
		ctx,
	}, nil
}

// AddPeer is used to add a remote peer to our peerstore
func (h *Host) AddPeer(peerAddr string) (peer.ID, error) {
	// create multiformat address
	maAddr, err := ma.NewMultiaddr(peerAddr)
	if err != nil {
		return "", err
	}
	// get p2p peer id
	pid, err := maAddr.ValueForProtocol(ma.P_P2P)
	if err != nil {
		return "", err
	}
	// decode peer id
	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		return "", err
	}
	// add peer to our peerstore permanently
	h.Peerstore().AddAddr(
		peerid, maAddr, pstore.PermanentAddrTTL,
	)
	return peerid, nil
}

// GetPeers is used to get a slice of all known peers
func (h *Host) GetPeers() peer.IDSlice {
	return h.Peerstore().Peers()
}

// GetPeerID is used to return the peers ID
func (h *Host) GetPeerID() (peer.ID, error) {
	return peer.IDFromPublicKey(h.pk.GetPublic())
}
