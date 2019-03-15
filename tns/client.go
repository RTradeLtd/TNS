package tns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	host "github.com/RTradeLtd/tns/host"
	pb "github.com/RTradeLtd/tns/tns/pb"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	dnsaddr "github.com/multiformats/go-multiaddr-dns"
)

const (
	defaultZoneName           = "myzone"
	defaultZoneManagerKeyName = "postables-3072"
	defaultZoneKeyName        = "postables-testkeydemo"
	defaultZoneUserName       = "postables"
	defaultRecordName         = "myrecord"
	defaultRecordKeyName      = "postables-testkeydemo2"
	defaultRecordUserName     = "postables"
	dev                       = true
)

// ClientOptions is used to configure the client during ininitialization
type ClientOptions struct {
	GenPK      bool
	PK         ci.PrivKey
	ListenAddr string
}

// Client is used to talk to a TNS daemon node
type Client struct {
	H *host.Host
}

// NewClient is used to instantiate a TNS Client
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	var (
		privateKey ci.PrivKey
		err        error
	)
	// allow the client to provide the crytographic identity to be used, or generate one
	if opts.GenPK {
		privateKey, _, err = ci.GenerateKeyPair(ci.Ed25519, 256)
		if err != nil {
			return nil, err
		}
	} else {
		privateKey = opts.PK
	}
	lHost, err := host.NewHost(ctx, privateKey, opts.ListenAddr)
	if err != nil {
		return nil, err
	}
	return &Client{
		H: lHost,
	}, nil
}

// Query is used to send a query to the tns daemon running at peerid
// TODO: use a response protobuf to contain a message
func (c *Client) Query(ctx context.Context, peerid peer.ID, proto Command, cmd *pb.Command) (interface{}, error) {
	requestBytes, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	// create a stream with the peer for the specified protocol
	// this will allow us to send/receive data
	stream, err := c.H.NewStream(ctx, peerid, proto.ID)
	if err != nil {
		return nil, err
	}
	// newline is used to signal the end of the request
	if bytesWritten, err := stream.Write(append(requestBytes, '\n')); err != nil {
		return nil, err
	} else if bytesWritten <= 0 {
		return nil, errors.New("unknown error ocurred while writing request")
	}
	// read response from peer
	response, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	// TODO: decode response into struct and return struct
	fmt.Println(string(response))
	return nil, nil
}

// ID is used to get the peerID of this client
func (c *Client) ID() peer.ID {
	return c.H.ID()
}

// Close is used to close our libp2p host
func (c *Client) Close() error {
	return c.H.Close()
}

// AddPeer is used to add a remote peer to our peerstore when we know its multiaddr
func (c *Client) AddPeer(peerAddress string) (peer.ID, error) {
	return c.H.AddPeer(peerAddress)
}

// FindPeer is used to find out the multiaddr/peer to connect to based off a _dnsaddr txt record
// we resolve the _dnsaddr record, and then connect to the first available address
func (c *Client) FindPeer(ctx context.Context, domain string) (peer.ID, error) {
	maAddr, err := ma.NewMultiaddr(domain)
	if err != nil {
		return "", err
	}
	addrs, err := dnsaddr.Resolve(ctx, maAddr)
	if err != nil {
		return "", err
	}
	return c.AddPeer(addrs[0].String())
}
