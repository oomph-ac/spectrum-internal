package session

import (
	"sync"

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

var pkCtxPool = sync.Pool{
	New: func() any {
		return &PacketContext{}
	},
}

// PacketContext represents the context of a packet being processed. It holds the state of whether the packet has been
// canceled and if the packet has been modified by the processor.
type PacketContext struct {
	canceled bool
	modified bool

	raw     []byte
	decoded packet.Packet
}

func NewPacketContext(raw []byte, decoded packet.Packet) *PacketContext {
	ctx := pkCtxPool.Get().(*PacketContext)
	ctx.raw = raw
	ctx.decoded = decoded
	return ctx
}

func ReturnPacketContext(ctx *PacketContext) {
	ctx.raw = nil
	ctx.decoded = nil
	ctx.modified = false
	ctx.canceled = false
	pkCtxPool.Put(ctx)
}

// Packet returns the underlying packet data of the context. This is only non-null if the packet is specified to be decoded.
func (ctx *PacketContext) Packet() packet.Packet {
	return ctx.decoded
}

// Payload returns the raw payload of the packet. This can be nil in the case where the packet is received by the server and
// already is decoded. If a packet from the remote server isn't decoded, this will be present.
func (ctx *PacketContext) Payload() []byte {
	return ctx.raw
}

// Cancel marks the context as canceled. This function is used to stop further processing of an action.
func (ctx *PacketContext) Cancel() {
	ctx.canceled = true
}

// Cancelled returns whether the context has been cancelled.
func (ctx *PacketContext) Cancelled() bool {
	return ctx.canceled
}

// Modified returns whether the packet has been modified by the processor.
func (ctx *PacketContext) Modified() bool {
	return ctx.modified
}

// SetModified marks the packet as modified by the processor.
func (ctx *PacketContext) SetModified() {
	ctx.modified = true
}

// Processor defines methods for processing various actions within a proxy session.
type Processor interface {
	// ProcessStartGame is called only once during the login sequence.
	ProcessStartGame(ctx *Context, data *minecraft.GameData)
	// ProcessServer is called before forwarding the server-sent packets to the client.
	ProcessServer(ctx *PacketContext)
	// ProcessClient is called before forwarding the client-sent packets to the server.
	ProcessClient(batch []*PacketContext)
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
func (NopProcessor) ProcessServer(_ *PacketContext)                          {}
func (NopProcessor) ProcessClient(_ []*PacketContext)                        {}
func (NopProcessor) ProcessFlush(_ *Context)                                 {}
func (NopProcessor) ProcessPreTransfer(_ *Context, _ *string, _ *string)     {}
func (NopProcessor) ProcessTransferFailure(_ *Context, _ *string, _ *string) {}
func (NopProcessor) ProcessPostTransfer(_ *Context, _ *string, _ *string)    {}
func (NopProcessor) ProcessCache(_ *Context, _ *[]byte)                      {}
func (NopProcessor) ProcessDisconnection(_ *Context, _ *string)              {}
