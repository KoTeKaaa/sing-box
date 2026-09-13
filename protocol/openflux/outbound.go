package openflux

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"universal-bypass-tool/transport"
	"universal-bypass-tool/transport/yandex"
	"universal-bypass-tool/tunnel"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.OpenFluxOutboundOptions](
		registry,
		constant.TypeOpenFlux,
		NewOutbound,
	)
}

var _ adapter.Outbound = (*Outbound)(nil)

type Outbound struct {
	outbound.Adapter
	logger log.ContextLogger

	trans  transport.Transport
	tunnel *tunnel.TCPTunnel
}

func NewOutbound(
	ctx context.Context,
	router adapter.Router,
	logger log.ContextLogger,
	tag string,
	options option.OpenFluxOutboundOptions,
) (adapter.Outbound, error) {
	_ = ctx
	_ = router

	if options.URL == "" {
		return nil, E.New("openflux: url is required")
	}

	// Точно та же цепочка, которую мы проверили
	// через cmd/ofprobe.
	cfg := transport.DefaultConfig()

	inner := yandex.NewYandexVolgaTransport(
		options.URL,
		cfg,
	)

	trans := transport.NewCompressedTransport(inner)

	if err := trans.Start(); err != nil {
		return nil, E.Cause(err, "openflux: start transport")
	}

	tun := tunnel.NewTCPTunnel(trans, false)

	logger.Info("OpenFlux outbound started")

	return &Outbound{
		Adapter: outbound.NewAdapter(
			constant.TypeOpenFlux,
			tag,
			[]string{N.NetworkTCP},
			nil,
		),
		logger: logger,
		trans:  trans,
		tunnel: tun,
	}, nil
}

func (o *Outbound) DialContext(
	ctx context.Context,
	network string,
	destination M.Socksaddr,
) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)

	metadata.Outbound = o.Tag()
	metadata.Destination = destination

	switch N.NetworkName(network) {
	case N.NetworkTCP:
		o.logger.InfoContext(
			ctx,
			"OpenFlux TCP connection to ",
			destination,
		)

		return o.tunnel.DialTCP(destination.String())

	default:
		return nil, E.New("openflux: only TCP is supported")
	}
}

func (o *Outbound) ListenPacket(
	ctx context.Context,
	destination M.Socksaddr,
) (net.PacketConn, error) {
	_ = ctx
	_ = destination

	return nil, E.New("openflux: UDP is not implemented")
}

func (o *Outbound) Close() error {
	if o.trans != nil {
		return o.trans.Stop()
	}

	return nil
}
