package ntsnet

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
	"sinanmohd.com/namescale/internal/config"
	"tailscale.com/client/local"
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
