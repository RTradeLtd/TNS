package tns

import (
	ci "github.com/libp2p/go-libp2p-crypto"
)

const (
	// CommandEcho is a test command used to test if we have successfully connected to a tns daemon
	CommandEcho = "/echo/1.0.0"
	// CommandRecordRequest is a command used to request a record from tns
	CommandRecordRequest = "/recordRequest/1.0.0"
	// CommandZoneRequest is a command used to request a zone from tns
	CommandZoneRequest = "/zoneRequest/1.0.0"
)

var (
	// Commands are all the commands that TNS supports via the libp2p interface
	Commands = []string{CommandEcho, CommandRecordRequest, CommandZoneRequest}
)

// RecordRequest is a message sent when requeting a record form TNS, the response is simply Record
type RecordRequest struct {
	RecordName string `json:"record_name"`
	UserName   string `json:"user_name"`
}

// ZoneRequest is a message sent when requesting a reccord from TNS.
type ZoneRequest struct {
	UserName           string `json:"user_name"`
	ZoneName           string `json:"zone_name"`
	ZoneManagerKeyName string `json:"zone_manager_key_name"`
}

// ZoneCreation is used to create a tns zone
type ZoneCreation struct {
	Name           string `json:"name"`
	ManagerKeyName string `json:"manager_key_name"`
	ZoneKeyName    string `json:"zone_key_name"`
}

// RecordCreation is used to create a tns record
type RecordCreation struct {
	ZoneName      string                 `json:"zone_name"`
	RecordName    string                 `json:"record_name"`
	RecordKeyName string                 `json:"record_key_name"`
	MetaData      map[string]interface{} `json:"meta_data"`
}

// Zone is a mapping of human readable names, mapped to a public key. In order to retrieve the latest
type Zone struct {
	Manager   *ZoneManager `json:"zone_manager"`
	PublicKey string       `json:"zone_public_key"`
	// A human readable name for this zone
	Name string `json:"name"`
	// A map of records managed by this zone
	Records                 map[string]*Record `json:"records"`
	RecordNamesToPublicKeys map[string]string  `json:"record_names_to_public_keys"`
}

// Record is a particular name entry managed by a zone
type Record struct {
	PublicKey string `json:"public_key"`
	// A human readable name for this record
	Name string `json:"name"`
	// User configurable meta data for this record
	MetaData map[string]interface{} `json:"meta_data"`
}

// ZoneManager is the authorized manager of a zone
type ZoneManager struct {
	PublicKey string `json:"public_key"`
}

// HostOpts is our options for when we create our libp2p host
type HostOpts struct {
	IPAddress string `json:"ip_address"`
	Port      string `json:"port"`
	IPVersion string `json:"ip_version"`
	Protocol  string `json:"protocol"`
}

// TNS is an interface used by a TNS client or daemon
type TNS interface {
	MakeHost(pk ci.PrivKey, opts *HostOpts) error
}
