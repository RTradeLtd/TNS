package tns

import (
	"context"
	"errors"
	"io/ioutil"
	"time"

	"github.com/RTradeLtd/rtfs"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
)

// GenerateStreamAndWrite is a helper function used to generate, and interact with a stream
func (c *Client) GenerateStreamAndWrite(ctx context.Context, peerID peer.ID, cmd, ipfsAPI string, reqBytes []byte) (interface{}, error) {
	var (
		s    inet.Stream
		intf interface{}
		err  error
	)
	switch cmd {
	case "record-record":
		s, err = c.H.NewStream(ctx, peerID, CommandRecordRequest)
	case "zone-request":
		s, err = c.H.NewStream(ctx, peerID, CommandZoneRequest)
	case "echo":
		s, err = c.H.NewStream(ctx, peerID, CommandEcho)
	default:
		return nil, errors.New("unsupported command")
	}
	if err != nil {
		return nil, err
	}
	// send a message
	_, err = s.Write(append(reqBytes, '\n'))
	if err != nil {
		return nil, err
	}
	// read the message
	resp, err := ioutil.ReadAll(s)
	if err != nil {
		return nil, err
	}
	if cmd == "echo" {
		return string(resp), nil
	}
	rtfsManager, err := rtfs.NewManager(ipfsAPI, "", time.Minute*10)
	if err != nil {
		return nil, err
	}
	if err = rtfsManager.DagGet(string(resp), &intf); err != nil {
		return nil, err
	}
	return intf, nil
}
