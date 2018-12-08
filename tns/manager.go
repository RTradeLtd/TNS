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

// ManagerOpts defines options for controlling our TNS Manager daemon
type ManagerOpts struct {
	ManagerPK ci.PrivKey `json:"manager_pk"`
	LogFile   string     `json:"log_file"`
	DB        *gorm.DB   `json:"db"`
	ipfs      rtfs.Manager
	kbc       *kaas.Client
}

// NewDaemon is used to create a new tns manager daemon
func NewDaemon(opts *ManagerOpts, db *gorm.DB, kbc *kaas.Client, ipfs rtfs.Manager) (*Daemon, error) {
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
		ID:         managerPKID,
		PrivateKey: opts.ManagerPK,
		kbc:        kbc,
		ipfs:       ipfs,
		ZM:         models.NewZoneManager(db),
		RM:         models.NewRecordManager(db),
		Zones:      make(map[string]string),
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

// RunTNSDaemon is used to run our TNS daemon, and setup the available stream handlers
func (d *Daemon) RunTNSDaemon() {
	d.LogInfo("generating echo stream")
	// our echo stream is a basic test used to determine whether or not a tns manager daemon is functioning properly
	d.Host.SetStreamHandler(
		CommandEcho, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "echo"); err != nil {
				log.Warn(err.Error())
				s.Reset()
			} else {
				s.Close()
			}
		})
	d.LogInfo("generating record request stream")
	// our record request stream allows clients to request a record from the tns manager daemon
	d.Host.SetStreamHandler(
		CommandRecordRequest, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "record-request"); err != nil {
				log.Warn(err.Error())
				s.Reset()
			} else {
				s.Close()
			}
		})
	d.LogInfo("generating zone request stream")
	// our zone request stream allows clients to request a zone from the tns manager daemon
	d.Host.SetStreamHandler(
		CommandZoneRequest, func(s net.Stream) {
			d.LogInfo("new stream detected")
			if err := d.HandleQuery(s, "zone-request"); err != nil {
				log.Warn(err.Error())
				s.Reset()
			} else {
				s.Close()
			}
		})
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
		if d.Zones[req.ZoneName] == "" {
			_, err = s.Write([]byte("daemon is not a manager for this zone"))
			return err
		}
		// send the latest ipld hash for this zone to the client, allowing them to extract information from ipfs
		_, err = s.Write([]byte(d.Zones[req.ZoneName]))
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
	d.Host = host
	return nil
}

// HostMultiAddress is used to get a formatted libp2p host multi address
func (d *Daemon) HostMultiAddress() (ma.Multiaddr, error) {
	return ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", d.Host.ID().Pretty()))
}

// ReachableAddress is used to get a reachable address for this host
func (d *Daemon) ReachableAddress(addressIndex int) (string, error) {
	if addressIndex > len(d.Host.Addrs()) {
		return "", errors.New("invalid index")
	}
	ipAddr := d.Host.Addrs()[addressIndex]
	multiAddr, err := d.HostMultiAddress()
	if err != nil {
		return "", err
	}
	return ipAddr.Encapsulate(multiAddr).String(), nil
}

// CreateZone is used to create a zone, storing it on ipfs
// Uses the embedded private key as the zone manager private key
func (d *Daemon) CreateZone(req *ZoneCreation) (string, error) {
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
	d.Zones[req.Name] = hash
	return hash, nil
}
