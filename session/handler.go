package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	spectrumpacket "github.com/cooldogedev/spectrum/server/packet"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// handleServer continuously reads packets from the server and forwards them to the client.
func handleServer(s *Session) {
loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		default:
		}

		server := s.Server()
		pk, err := server.ReadPacket()
		if err != nil {
			if server != s.Server() {
				continue loop
			}

			server.CloseWithError(fmt.Errorf("failed to read packet from server: %w", err))
			if err := s.fallback(); err != nil {
				s.CloseWithError(fmt.Errorf("fallback failed: %w", err))
				break loop
			}
			continue loop
		}

		switch pk := pk.(type) {
		case *spectrumpacket.Flush:
			ctx := NewContext()
			s.Processor().ProcessFlush(ctx)
			if ctx.Cancelled() {
				continue loop
			}

			if err := s.client.Flush(); err != nil {
				s.CloseWithError(fmt.Errorf("failed to flush client's buffer: %w", err))
				logError(s, "failed to flush client's buffer", err)
				break loop
			}
		case *spectrumpacket.Latency:
			s.latency.Store(pk.Latency)
		case *spectrumpacket.Transfer:
			if err := s.Transfer(pk.Addr); err != nil {
				logError(s, "failed to transfer", err)
			}
		case *spectrumpacket.UpdateCache:
			s.SetCache(pk.Cache)
		case packet.Packet:
			ctx := NewPacketContext(nil, pk)
			s.Processor().ProcessServer(ctx)
			if ctx.Cancelled() {
				continue loop
			}

			if s.opts.SyncProtocol {
				for _, latest := range s.client.Proto().ConvertToLatest(pk, s.client) {
					s.tracker.handlePacket(latest)
				}
			} else {
				s.tracker.handlePacket(pk)
			}
			if err := s.client.WritePacket(pk); err != nil {
				s.CloseWithError(fmt.Errorf("failed to write packet to client: %w", err))
				logError(s, "failed to write packet to client", err)
				break loop
			}
		case []byte:
			ctx := NewPacketContext(pk, nil)
			s.Processor().ProcessServer(ctx)
			if ctx.Cancelled() {
				continue loop
			}

			if _, err := s.client.Write(pk); err != nil {
				s.CloseWithError(fmt.Errorf("failed to write packet to client: %w", err))
				logError(s, "failed to write packet to client", err)
				break loop
			}
		}
	}
}

// handleClient continuously reads packets from the client and forwards them to the server.
func handleClient(s *Session) {
	header := &packet.Header{}
	pool := s.client.Proto().Packets(true)
	var shieldID int32
	for _, item := range s.client.GameData().Items {
		if item.Name == "minecraft:shield" {
			shieldID = int32(item.RuntimeID)
			break
		}
	}

loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		default:
		}

		payloads, err := s.client.ReadBatchBytes()
		if err != nil {
			s.CloseWithError(fmt.Errorf("failed to read packet from client: %w", err))
			logError(s, "failed to read packet from client", err)
			break loop
		}
		/* for _, payload := range payloads {
			if err := handleClientPacketLegacy(s, header, pool, shieldID, payload); err != nil {
				s.Server().CloseWithError(fmt.Errorf("failed to write packet to server: %w", err))
			}
		} */
		if err := handleClientBatch(s, header, pool, shieldID, payloads); err != nil {
			s.Server().CloseWithError(fmt.Errorf("failed to write packet to server: %w", err))
			logError(s, "failed to write packet to server", err)
			break loop
		}
	}
}

// handleLatency periodically sends the client's current ping and timestamp to the server for latency reporting.
// The client's latency is derived from half of RakNet's round-trip time (RTT).
// To calculate the total latency, we multiply this value by 2.
func handleLatency(s *Session, interval int64) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer ticker.Stop()
loop:
	for {
		select {
		case <-s.ctx.Done():
			s.CloseWithError(context.Cause(s.ctx))
			break loop
		case <-ticker.C:
			if err := s.Server().WritePacket(&spectrumpacket.Latency{Latency: s.client.Latency().Milliseconds() * 2, Timestamp: time.Now().UnixMilli()}); err != nil {
				logError(s, "failed to write latency packet", err)
			}
		}
	}
}

func handleClientBatch(s *Session, header *packet.Header, pool packet.Pool, shieldID int32, payloads [][]byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while handling batch: %v", r)
		}
	}()

	ctxBatch := make([]*PacketContext, 0, len(payloads))
	for _, payload := range payloads {
		ctx, err := decodeAndCreateContext(s, header, pool, shieldID, payload)
		if err != nil {
			return err
		} else if ctx == nil {
			continue
		}
		ctxBatch = append(ctxBatch, ctx)
	}

	s.Processor().ProcessClient(ctxBatch)
	var (
		proto        minecraft.Protocol
		payloadBatch = make([][]byte, 0, len(ctxBatch))
	)

	if s.opts.SyncProtocol {
		proto = s.client.Proto()
	} else {
		proto = minecraft.DefaultProtocol
	}
	for _, ctx := range ctxBatch {
		if ctx.Cancelled() {
			ReturnPacketContext(ctx)
			continue
		}

		if ctx.decoded == nil || !ctx.Modified() {
			payloadBatch = append(payloadBatch, ctx.raw)
			ReturnPacketContext(ctx)
			continue
		}

		// If the packet was modified, we have to re-encode the packet, and then append that to the payload batch.
		newPkBuf := bytes.NewBuffer(nil)
		w := proto.NewWriter(newPkBuf, shieldID)
		header.PacketID = ctx.decoded.ID()
		_ = header.Write(newPkBuf)
		ctx.decoded.Marshal(w)
		payloadBatch = append(payloadBatch, newPkBuf.Bytes())
	}
	return s.Server().WriteBatch(payloadBatch)
}

// decodeAndCreateContext decodes a client packet and either returns packet.Packet if decode is specified, or a byte slice if decode is not needed. It also will
// return any errors that occur while decoding this packet. If SyncProtocol is disabled and the session needs packets to be upgraded, we've made the assumption that
// only the first packet from the upgraded array needs to be handled, as there aren't any cases known yet where it's actually neccessary for the client specifically.
// If the client is not on the latest version and SyncProtocol is disabled, we will not ever allow for a packet to not be decoded and pass through raw. This is because
// forwarding a raw legacy packet to a downstream likely equipped without multi-version support would lead to decoding errors.
func decodeAndCreateContext(s *Session, header *packet.Header, pool packet.Pool, shieldID int32, payload []byte) (ctx *PacketContext, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while decoding packet from client batch: %v", r)
		}
	}()

	buf := bytes.NewBuffer(payload)
	if err := header.Read(buf); err != nil {
		return nil, errors.New("failed to decode header")
	}

	pkFunc, ok := pool[header.PacketID]
	if !ok {
		return nil, fmt.Errorf("unknown packet with id %d", header.PacketID)
	}

	// If SyncProtocol is disabled, and the client is not on the latest version, we need to decode the packet. If we don't, this can lead to
	// issues where we forward a legacy version packet to the downstream server, resulting in decoding errors.
	isClientLatestVersion := s.client.Proto().ID() == protocol.CurrentProtocol
	if !slices.Contains(s.opts.ClientDecode, header.PacketID) && (s.opts.SyncProtocol || isClientLatestVersion) {
		return NewPacketContext(payload, nil), nil
	}

	decodedPk := pkFunc()
	decodedPk.Marshal(s.client.Proto().NewReader(buf, shieldID, true))
	if extra := buf.Len(); extra > 0 {
		return nil, fmt.Errorf("%T had an extra %d bytes", decodedPk, extra)
	}

	// If we are not using SyncProtocol, we should upgrade the packet to the latest version. For now, we will ignore extra packets
	// returned by the protocol library as there aren't any packets that require it at the moment.
	if !s.opts.SyncProtocol && !isClientLatestVersion {
		upgraded := s.client.Proto().ConvertToLatest(decodedPk, s.client)
		if len(upgraded) == 0 {
			return nil, nil
		}
		return NewPacketContext(payload, upgraded[0]), nil
	}
	return NewPacketContext(payload, decodedPk), nil
}

func logError(s *Session, msg string, err error) {
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	if !errors.Is(err, context.Canceled) {
		s.logger.Error(msg, "err", err)
	}
}
