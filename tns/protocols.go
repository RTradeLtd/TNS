package tns

import (
	protocol "github.com/libp2p/go-libp2p-protocol"
)

// ProtocolID is a typed string used for libp2p protocol definition
// it satisfies protocol.ID requirements
type ProtocolID struct {
	protocol.ID
}

// String is used to return a string of ProtocolID
func (pid ProtocolID) String() string {
	return string(pid.ID)
}

// Command is a particular command we are running
// it satisfies protocol.ID requirements
type Command struct {
	Name string
	protocol.ID
}

// String is used to return a string of Command
// The string is also the name
func (c Command) String() string {
	return string(c.ID)
}

// GRPCID returns the ID of the command as per the proto buffer
func (c Command) GRPCID() int32 {
	switch c.Name {
	case "RECORD_REQUEST":
		return 0
	case "ZONE_REQUEST":
		return 1
	case "ECHO":
		return 2
	default:
		return -1
	}
}

var (
	// TNS is the Temporal Name Server Protocol
	TNS = ProtocolID{protocol.ID("/tns/0.0.1")}
	// CommandRecordRequest is used to request a particular request to resolve (ie web.example.com)
	CommandRecordRequest = Command{Name: "RECORD_REQUEST", ID: protocol.ID("/record/request/0.0.1")}
	// CommandZoneRequest is used to request information about a zone (ie example.com)
	CommandZoneRequest = Command{Name: "ZONE_REQUEST", ID: protocol.ID("/zone/request/0.0.1")}
	// CommandEcho is a generic command used to test connectivity
	CommandEcho = Command{Name: "ECHO", ID: protocol.ID("/echo/0.0.1")}
	// Commands are all the possible commands you could run
	Commands = []Command{CommandRecordRequest, CommandZoneRequest, CommandEcho}
)
