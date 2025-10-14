package session

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Context represents the context of an action. It holds the state of whether the action has been canceled.
type Context struct {
	canceled bool
}

// NewContext returns a new context.
func NewContext() *Context {
	return &Context{}
}

// Cancel marks the context as canceled. This function is used to stop further processing of an action.
func (c *Context) Cancel() {
	c.canceled = true
}

// Cancelled returns whether the context has been cancelled.
func (c *Context) Cancelled() bool {
	return c.canceled
}

// PacketContext represents the context of a packet being processed. It holds the state of whether the packet has been
// canceled and if the packet has been modified by the processor.
type PacketContext struct {
	canceled bool
	modified bool
}

func NewPacketContext() *PacketContext {
	return &PacketContext{}
}

// Cancel marks the context as canceled. This function is used to stop further processing of an action.
func (c *PacketContext) Cancel() {
	c.canceled = true
}

// Cancelled returns whether the context has been cancelled.
func (c *PacketContext) Cancelled() bool {
	return c.canceled
}

// Modified returns whether the packet has been modified by the processor.
func (c *PacketContext) Modified() bool {
	return c.modified
}

// SetModified marks the packet as modified by the processor.
func (c *PacketContext) SetModified() {
	c.modified = true
}

// Processor defines methods for processing various actions within a proxy session.
type Processor interface {
	// ProcessStartGame is called only once during the login sequence.
	ProcessStartGame(ctx *Context, data *minecraft.GameData)
	// ProcessServer is called before forwarding the server-sent packets to the client.
	ProcessServer(ctx *PacketContext, pk *packet.Packet)
	// ProcessServerEncoded is called before forwarding the server-sent packets to the client.
	ProcessServerEncoded(ctx *PacketContext, pk *[]byte)
	// ProcessClient is called before forwarding the client-sent packets to the server.
	ProcessClient(ctx *PacketContext, pk *packet.Packet)
	// ProcessClientEncoded is called before forwarding the client-sent packets to the server.
	ProcessClientEncoded(ctx *PacketContext, pk *[]byte)
	// ProcessFlush is called before flushing the player's minecraft.Conn buffer in response to a downstream server request.
	ProcessFlush(ctx *Context)
	// ProcessPreTransfer is called before transferring the player to a different server.
	ProcessPreTransfer(ctx *Context, origin *string, target *string)
	// ProcessTransferFailure is called when the player transfer to a different server fails.
	ProcessTransferFailure(ctx *Context, origin *string, target *string)
	// ProcessPostTransfer is called after transferring the player to a different server.
	ProcessPostTransfer(ctx *Context, origin *string, target *string)
	// ProcessCache is called before updating the session's cache.
	ProcessCache(ctx *Context, new *[]byte)
	// ProcessDisconnection is called when the player disconnects from the proxy.
	ProcessDisconnection(ctx *Context, message *string)
}

// NopProcessor is a no-operation implementation of the Processor interface.
type NopProcessor struct{}

// Ensure that NopProcessor satisfies the Processor interface.
var _ Processor = NopProcessor{}

func (NopProcessor) ProcessStartGame(_ *Context, _ *minecraft.GameData)      {}
func (NopProcessor) ProcessServer(_ *PacketContext, _ *packet.Packet)        {}
func (NopProcessor) ProcessServerEncoded(_ *PacketContext, _ *[]byte)        {}
func (NopProcessor) ProcessClient(_ *PacketContext, _ *packet.Packet)        {}
func (NopProcessor) ProcessClientEncoded(_ *PacketContext, _ *[]byte)        {}
func (NopProcessor) ProcessFlush(_ *Context)                                 {}
func (NopProcessor) ProcessPreTransfer(_ *Context, _ *string, _ *string)     {}
func (NopProcessor) ProcessTransferFailure(_ *Context, _ *string, _ *string) {}
func (NopProcessor) ProcessPostTransfer(_ *Context, _ *string, _ *string)    {}
func (NopProcessor) ProcessCache(_ *Context, _ *[]byte)                      {}
func (NopProcessor) ProcessDisconnection(_ *Context, _ *string)              {}
