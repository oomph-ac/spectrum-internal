package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

// EOBNotification is a packet sent by the server when the end of a packet batch is reached.
type EOBNotification struct{}

func (*EOBNotification) ID() uint32 {
	return IDEOBNotification
}

func (*EOBNotification) Marshal(io protocol.IO) {}
