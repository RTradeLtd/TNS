package tns

import (
	"context"
	"encoding/json"
	"errors"

	host "github.com/RTradeLtd/tns/host"
	ci "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
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

// Client is used to query a TNS daemon
type Client struct {
	H       *host.Host
	IPFSAPI string
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

// Close is used to close our libp2p host
func (c *Client) Close() error {
	return c.H.Close()
}

// QueryTNS is used to query a peer for TNS name resolution
func (c *Client) QueryTNS(peerID peer.ID, cmd string, requestArgs interface{}) (interface{}, error) {
	switch cmd {
	case "echo":
		// send a basic echo test
		return c.queryEcho(peerID)
	case "zone-request":
		// ensure the request argument is of type zone request
		args := requestArgs.(ZoneRequest)
		return c.ZoneRequest(peerID, &args)
	case "record-request":
		args := requestArgs.(RecordRequest)
		return c.RecordRequest(peerID, &args)
	default:
		return nil, errors.New("unsupported cmd")
	}
}

// ZoneRequest is a call used to request a zone from TNS
func (c *Client) ZoneRequest(peerID peer.ID, req *ZoneRequest) (interface{}, error) {
	if req == nil {
		req = &ZoneRequest{
			ZoneName:           defaultZoneName,
			ZoneManagerKeyName: defaultZoneManagerKeyName,
			UserName:           defaultZoneUserName,
		}
	}
	marshaledData, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}
	return c.GenerateStreamAndWrite(
		context.Background(), peerID, "zone-request", c.IPFSAPI, marshaledData,
	)
}

// RecordRequest is a call used to request a record from TNS
func (c *Client) RecordRequest(peerID peer.ID, req *RecordRequest) (interface{}, error) {
	if req == nil {
		req = &RecordRequest{
			RecordName: defaultRecordName,
			UserName:   defaultRecordUserName,
		}
	}
	marshaledData, err := json.Marshal(&req)
	if err != nil {
		return nil, err
	}
	return c.GenerateStreamAndWrite(
		context.Background(), peerID, "record-request", c.IPFSAPI, marshaledData,
	)
}

func (c *Client) queryEcho(peerID peer.ID) (interface{}, error) {
	return c.GenerateStreamAndWrite(
		context.Background(), peerID, "echo", c.IPFSAPI, []byte("test\n"),
	)
}
