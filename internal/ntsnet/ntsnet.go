package ntsnet

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/miekg/dns"
	"sinanmohd.com/namescale/internal/config"
	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

type Ntsnet struct {
	localClient *local.Client
	TsnetServer *tsnet.Server
}

func New(cfg config.Tsnet) (*Ntsnet, error) {
	var namescaleTsnet Ntsnet

	srv := new(tsnet.Server)
	srv.Hostname = cfg.Hostname
	srv.Port = cfg.Port
	srv.AuthKey = cfg.AuthKey
	srv.ControlURL = cfg.CoordinationServerURL
	srv.AdvertiseTags = []string{"namescale"}
	srv.Ephemeral = cfg.Ephemeral
	namescaleTsnet.TsnetServer = srv

	localClient, err := srv.LocalClient()
	if err != nil {
		return nil, err
	}
	namescaleTsnet.localClient = localClient

	_, err = srv.Up(context.Background())
	if err != nil {
		return nil, err
	}

	err = namescaleTsnet.sanityCheck(context.Background())
	if err != nil {
		return nil, err
	}

	return &namescaleTsnet, nil
}

func (t *Ntsnet) sanityCheck(ctx context.Context) error {
	_, err := t.BaseDomainGet(ctx)
	return err
}

func (t *Ntsnet) BaseDomainGet(ctx context.Context) (string, error) {
	status, err := t.localClient.Status(ctx)
	if err != nil {
		return "", err
	}

	if !status.CurrentTailnet.MagicDNSEnabled {
		return "", fmt.Errorf("MagicDNSEnabled is disabled on Tailnet %s", status.CurrentTailnet.Name)
	}

	return dns.Fqdn(status.CurrentTailnet.MagicDNSSuffix), nil
}

func (t *Ntsnet) MagicDNSResolve(ctx context.Context, HostFQDN string, qType uint16) (*netip.Addr, bool, error) {
	status, err := t.localClient.Status(ctx)
	if err != nil {
		return nil, false, err
	}

	if !status.CurrentTailnet.MagicDNSEnabled {
		return nil, false, fmt.Errorf("MagicDNSEnabled is disabled on Tailnet %s", status.CurrentTailnet.Name)
	}

	var peer *ipnstate.PeerStatus
	for _, p := range status.Peer {
		if p.DNSName != HostFQDN {
			continue
		}
		peer = p
		break
	}
	if peer == nil {
		return nil, false, nil
	}

	idx := slices.IndexFunc(peer.TailscaleIPs, func(ip netip.Addr) bool {
		if ip.Is6() && qType == dns.TypeAAAA {
			return true
		} else if ip.Is4() && qType == dns.TypeA {
			return true
		}

		return false
	})
	if idx == -1 {
		return nil, false, nil
	}

	return &peer.TailscaleIPs[idx], true, nil
}
