package tns

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/RTradeLtd/rtfs"

	"github.com/RTradeLtd/database/models"
	pb "github.com/RTradeLtd/grpc/krab"
	"github.com/RTradeLtd/kaas"
	"github.com/jinzhu/gorm"
	ci "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
)

// DaemonOpts defines options for controlling our TNS Manager daemon
type DaemonOpts struct {
	ManagerPK ci.PrivKey `json:"manager_pk"`
	LogFile   string     `json:"log_file"`
	DB        *gorm.DB   `json:"db"`
	IPFS      rtfs.Manager
	KBC       *kaas.Client
}

// NewDaemon is used to create a new tns manager daemon
func NewDaemon(opts *DaemonOpts) (*Daemon, error) {
	var (
		logger = log.New()
		err    error
	)
	// extract a peer id for the zone manager
	managerPKID, err := peer.IDFromPublicKey(opts.ManagerPK.GetPublic())
	if err != nil {
		return nil, err
	}
	daemon := Daemon{
		ID:    managerPKID,
		pk:    opts.ManagerPK,
		kbc:   opts.KBC,
		ipfs:  opts.IPFS,
		zm:    models.NewZoneManager(opts.DB),
		rm:    models.NewRecordManager(opts.DB),
		zones: make(map[string]string),
	}
	// create our libp2p host
	if err := daemon.MakeHost(opts.ManagerPK, nil); err != nil {
		return nil, err
	}
	// open log file
	logfile, err := os.OpenFile(opts.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %s", err)
	}

	logger.Out = logfile
	logger.Info("logger initialized")
	daemon.l = logger
	return &daemon, nil
}

// Run is used to run our TNS daemon, and setup the available stream handlers
func (d *Daemon) Run(ctx context.Context) error {
	d.LogInfo("generating echo stream")
	// our echo stream is a basic test used to determine whether or not a tns manager daemon is functioning properly
	d.host.SetStreamHandler(
		CommandEcho, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "echo"); err != nil {
				d.l.Warn(err.Error())
				s.Reset()
			} else {
				d.l.Info("successfully handled echo stream")
				s.Close()
			}
		})
	d.LogInfo("generating record request stream")
	// our record request stream allows clients to request a record from the tns manager daemon
	d.host.SetStreamHandler(
		CommandRecordRequest, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "record-request"); err != nil {
				d.l.Warn(err.Error())
				s.Reset()
			} else {
				d.l.Info("successfully handled record request stream")
				s.Close()
			}
		})
	d.LogInfo("generating zone request stream")
	// our zone request stream allows clients to request a zone from the tns manager daemon
	d.host.SetStreamHandler(
		CommandZoneRequest, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "zone-request"); err != nil {
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
func (d *Daemon) HandleQuery(s net.Stream, cmd string) error {
	responseBuffer := bufio.NewReader(s)
	switch cmd {
	case "echo":
		// read the message being sent by the client
		// it must end with a new line
		bodyBytes, err := responseBuffer.ReadString('\n')
		if err != nil {
			return err
		}
		// format a response
		msg := fmt.Sprintf("echo test...\nyou sent: %s\n", string(bodyBytes))
		// send a response to th eclient
		_, err = s.Write([]byte(msg))
		return err
	case "record-request":
		return errors.New("not yet implemented")
	case "zone-request":
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

// MakeHost is used to generate the libp2p connection for our TNS daemon
func (d *Daemon) MakeHost(pk ci.PrivKey, opts *HostOpts) error {
	host, err := makeHost(pk, opts, false)
	if err != nil {
		return err
	}
	d.host = host
	return nil
}

// HostMultiAddress is used to get a formatted libp2p host multi address
func (d *Daemon) HostMultiAddress() (ma.Multiaddr, error) {
	return ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", d.host.ID().Pretty()))
}

// ReachableAddress is used to get a reachable address for this host
func (d *Daemon) ReachableAddress(addressIndex int) (string, error) {
	if addressIndex > len(d.host.Addrs()) {
		return "", errors.New("invalid index")
	}
	ipAddr := d.host.Addrs()[addressIndex]
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
		d.LogError(err, "failed to add zone to database")
	}
	d.zones[req.Name] = hash
	return hash, nil
}

// Close is used to terminate our daemon
func (d *Daemon) Close() error {
	return d.host.Close()
}

// Zones is used to retrieve all the zones being managed by this daemon
func (d *Daemon) Zones() map[string]string {
	return d.zones
}
