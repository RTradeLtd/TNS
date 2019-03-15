package tns

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RTradeLtd/config"
	"github.com/RTradeLtd/database"
	"github.com/RTradeLtd/rtfs"

	"github.com/RTradeLtd/database/models"
	pb "github.com/RTradeLtd/grpc/krab"
	"github.com/RTradeLtd/kaas"
	host "github.com/RTradeLtd/tns/host"
	"github.com/RTradeLtd/tns/log"
	ci "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// Options defines options for controlling our TNS Manager daemon
type Options struct {
	ManagerPK  ci.PrivKey
	Config     *config.TemporalConfig
	ListenAddr string
	Dev        bool
}

// Daemon is used to manage a local instance of a temporal name server
// A single Daemon (aka, manager) private key can be reused across multiple zones
type Daemon struct {
	ID peer.ID
	// pk is also our zone manager private key
	pk ci.PrivKey
	// zones is a map of zoneName -> latestIPLDHash
	zones map[string]string
	h     *host.Host
	zm    *models.ZoneManager
	rm    *models.RecordManager
	l     *zap.SugaredLogger
	kbc   *kaas.Client
	ipfs  rtfs.Manager
}

// NewDaemon is used to create a new tns manager daemon
func NewDaemon(ctx context.Context, opts Options) (*Daemon, error) {
	logger, err := log.NewLogger(opts.Config.LogFile, opts.Dev)
	if err != nil {
		return nil, err
	}
	kClient, err := kaas.NewClient(opts.Config.Services, false)
	if err != nil {
		return nil, err
	}
	ipfs, err := rtfs.NewManager(
		opts.Config.IPFS.APIConnection.Host+":"+opts.Config.IPFS.APIConnection.Port,
		"",
		time.Hour*1,
	)
	if err != nil {
		return nil, err
	}
	dbm, err := database.New(opts.Config, database.Options{SSLModeDisable: opts.Dev})
	if err != nil {
		return nil, err
	}
	// extract a peer id for the zone manager
	managerPKID, err := peer.IDFromPublicKey(opts.ManagerPK.GetPublic())
	if err != nil {
		return nil, err
	}
	daemon := Daemon{
		ID:    managerPKID,
		pk:    opts.ManagerPK,
		kbc:   kClient,
		ipfs:  ipfs,
		zm:    models.NewZoneManager(dbm.DB),
		rm:    models.NewRecordManager(dbm.DB),
		zones: make(map[string]string),
		l:     logger,
	}
	lHost, err := host.NewHost(ctx, opts.ManagerPK, opts.ListenAddr)
	if err != nil {
		return nil, err
	}
	daemon.h = lHost
	return &daemon, nil
}

// Run is used to run our TNS daemon, and setup the available stream handlers
func (d *Daemon) Run(ctx context.Context) error {
	d.l.Info("generating echo stream")
	// our echo stream is a basic test used to determine whether or not a tns manager daemon is functioning properly
	d.h.SetStreamHandler(
		CommandEcho.ID, func(s net.Stream) {
			d.l.Info("new stream detected")
			if err := d.HandleQuery(s, CommandEcho); err != nil {
				d.l.Warn(err.Error())
				s.Reset()
			} else {
				d.l.Info("successfully handled echo stream")
				s.Close()
			}
		})
	d.l.Info("generating record request stream")
	// our record request stream allows clients to request a record from the tns manager daemon
	d.h.SetStreamHandler(
		CommandRecordRequest.ID, func(s net.Stream) {
			d.l.Info("new stream detected")
			if err := d.HandleQuery(s, CommandRecordRequest); err != nil {
				d.l.Warn(err.Error())
				s.Reset()
			} else {
				d.l.Info("successfully handled record request stream")
				s.Close()
			}
		})
	d.l.Info("generating zone request stream")
	// our zone request stream allows clients to request a zone from the tns manager daemon
	d.h.SetStreamHandler(
		CommandZoneRequest.ID, func(s net.Stream) {
			d.l.Info("new stream detected")
			if err := d.HandleQuery(s, CommandZoneRequest); err != nil {
				d.l.Warn(err.Error())
				s.Reset()
			} else {
				d.l.Info("successfully handled zone request stream")
				s.Close()
			}
		})
	for {
		select {
		case <-ctx.Done():
			d.l.Info("terminating daemon")
			return d.Close()
		}
	}
}

// HandleQuery is used to handle a query sent to tns
func (d *Daemon) HandleQuery(s net.Stream, cmd Command) error {
	responseBuffer := bufio.NewReader(s)
	switch cmd {
	case CommandEcho:
		// read the message being sent by the client
		// it must end with a new line
		bodyBytes, err := responseBuffer.ReadString('\n')
		if err != nil {
			return err
		}
		// send a response to th eclient
		_, err = s.Write([]byte(fmt.Sprintf("echo test...\nyou sent: %s\n", string(bodyBytes))))
		return err
	case CommandRecordRequest:
		return errors.New("not yet implemented")
	case CommandZoneRequest:
		// read the message being sent by the client
		// it must end wit ha new line
		bodyBytes, err := responseBuffer.ReadBytes('\n')
		if err != nil {
			return err
		}
		// unmarshal the message into a zone request type
		req := ZoneRequest{}
		if err = json.Unmarshal(bodyBytes, &req); err != nil {
			return err
		}
		// check to see if we are managing the zone
		if d.zones[req.ZoneName] == "" {
			_, err = s.Write([]byte("daemon is not a manager for this zone"))
			return err
		}
		// send the latest ipld hash for this zone to the client, allowing them to extract information from ipfs
		_, err = s.Write([]byte(d.zones[req.ZoneName]))
		return err
	default:
		// basic handler for a generic stream
		_, err := s.Write([]byte("message received thanks"))
		return err
	}
}

// HostMultiAddress is used to get a formatted libp2p host multi address
func (d *Daemon) HostMultiAddress() (ma.Multiaddr, error) {
	return ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", d.h.ID().Pretty()))
}

// ReachableAddress is used to get a reachable address for this host
func (d *Daemon) ReachableAddress(addressIndex int) (string, error) {
	if addressIndex > len(d.h.Addrs()) {
		return "", errors.New("invalid index")
	}
	ipAddr := d.h.Addrs()[addressIndex]
	multiAddr, err := d.HostMultiAddress()
	if err != nil {
		return "", err
	}
	return ipAddr.Encapsulate(multiAddr).String(), nil
}

// CreateZone is used to create a zone, storing it on ipfs
// Uses the embedded private key as the zone manager private key
func (d *Daemon) CreateZone(req *ZoneCreation) (string, error) {
	if d.zones[req.Name] != "" {
		return "", errors.New("zone with name is already managed by this daemon")
	}
	// create zone private key
	zonePK, zonePubKey, err := ci.GenerateKeyPair(ci.Ed25519, 256)
	if err != nil {
		return "", err
	}
	zonePKBytes, err := zonePK.Bytes()
	if err != nil {
		return "", err
	}
	// store zone manager private key
	if _, err = d.kbc.PutPrivateKey(context.Background(), &pb.KeyPut{Name: req.ZoneKeyName, PrivateKey: zonePKBytes}); err != nil {
		return "", err
	}
	// convert public keys to ids
	zoneID, err := peer.IDFromPublicKey(zonePubKey)
	if err != nil {
		return "", err
	}
	// create the initial zone object
	z := Zone{
		PublicKey: zoneID.Pretty(),
		Manager: &ZoneManager{
			PublicKey: d.ID.Pretty(),
		},
		Name: req.Name,
	}
	// marshal into bytes
	marshaled, err := json.Marshal(&z)
	if err != nil {
		return "", err
	}
	hash, err := d.ipfs.DagPut(marshaled, "json", "cbor")
	if err != nil {
		return "", err
	}
	if _, err := d.zm.NewZone(req.Name, d.ID.Pretty(), zoneID.Pretty(), hash); err != nil {
		d.l.Errorw("failed to add zone to database", "error", err)
	}
	d.zones[req.Name] = hash
	return hash, nil
}

// PeerID is used to return the daemons peer ID
func (d *Daemon) PeerID() peer.ID {
	return d.h.ID()
}

// Close is used to terminate our daemon
func (d *Daemon) Close() error {
	return d.h.Close()
}

// Zones is used to retrieve all the zones being managed by this daemon
func (d *Daemon) Zones() map[string]string {
	return d.zones
}
