package dns

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"sinanmohd.com/namescale/internal/config"
	"sinanmohd.com/namescale/internal/ntsnet"
	"tailscale.com/tsnet"
)

const (
	ttl             = 512
	resolveconfPath = "/etc/resolv.conf"
	headscaleNs     = "100.100.100.100"
)

type Handler struct {
	dnsConfig *dns.ClientConfig
	ntsnet    *ntsnet.Ntsnet
}

func hostFqdnFromWildQustion(name, baseFqdn string) (string, error) {
	hasSufix := strings.HasSuffix(name, baseFqdn)
	if !hasSufix {
		return "", fmt.Errorf("Qustion name '%s' does not match baseDomain '%s'", name, baseFqdn)
	}

	ss := strings.Split(strings.TrimSuffix(name, baseFqdn), ".")
	if len(ss) < 2 || ss[len(ss)-2] == "" {
		return "", fmt.Errorf("Getting Host From name '%s", name)
	}

	return fmt.Sprintf("%s.%s", ss[len(ss)-2], baseFqdn), nil
}

func (handler *Handler) ServeFromRootNS(w dns.ResponseWriter, req *dns.Msg) {
	client := new(dns.Client)
	var resp *dns.Msg
	var err error

	for _, upstream := range handler.dnsConfig.Servers {
		if upstream == headscaleNs {
			continue
		}

		resp, _, err = client.Exchange(req, net.JoinHostPort(upstream, handler.dnsConfig.Port))
		if err == nil {
			w.WriteMsg(resp)
			return
		}

		slog.Error("Root NS resolving", "err", err)
	}

	w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
}

func (handler *Handler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	if req.Opcode != dns.OpcodeQuery {
		slog.Error("Ignoring non-query request", "name", req.Question[0].Name, "opcode", req.Opcode)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
		return
	}

	baseDomain, err := handler.ntsnet.BaseDomainGet(context.Background())
	if err != nil {
		slog.Error("Getting baseDomain", "err", err)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
		return
	}

	for i := range req.Question {
		// pass the base domain to root ns
		if req.Question[i].Name == baseDomain {
			handler.ServeFromRootNS(w, req)
			return
		}

		header := dns.RR_Header{
			Name:   req.Question[i].Name,
			Rrtype: req.Question[i].Qtype,
			Class:  req.Question[i].Qclass,
			Ttl:    ttl,
		}

		// handle the rest (wild card)
		hostFqdn, err := hostFqdnFromWildQustion(req.Question[i].Name, baseDomain)
		if err != nil {
			slog.Error("Getting hostFqdn", "err", err)
			w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
			return
		}

		ip, found, err := handler.ntsnet.MagicDNSResolve(context.Background(), hostFqdn, req.Question[i].Qtype)
		if err != nil {
			slog.Error("MagicDNS resolve", "err", err)
			w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
			return
		}
		if !found {
			w.WriteMsg(req.SetRcode(req, dns.RcodeNameError))
			return
		}

		switch req.Question[i].Qtype {
		case dns.TypeA:
			req.Answer = append(req.Answer, &dns.A{
				Hdr: header,
				A:   ip.AsSlice(),
			})
		case dns.TypeAAAA:
			req.Answer = append(req.Answer, &dns.AAAA{
				Hdr:  header,
				AAAA: ip.AsSlice(),
			})
		default:
			slog.Error("Unexpected qType", "qType", req.Question[i].Qtype)
			w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
			return
		}
	}

	w.WriteMsg(req)
}

func Serve(tsnetServer *tsnet.Server, handler *Handler) ([]*dns.Server, error) {
	ip4, ip6 := tsnetServer.TailscaleIPs()

	var servers []*dns.Server
	for _, ip := range []netip.Addr{ip4, ip6} {
		addr := net.JoinHostPort(ip.String(), "53")
		packetConn, err := tsnetServer.ListenPacket("udp", addr)
		if err != nil {
			return nil, err
		}
		udpSrv := dns.Server{
			Handler:    handler,
			PacketConn: packetConn,
		}
		go func() {
			err := udpSrv.ActivateAndServe()
			if err != nil {
				log.Fatal(err)
			}
		}()
		servers = append(servers, &udpSrv)

		listener, err := tsnetServer.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		tcpSrv := dns.Server{
			Handler:  handler,
			Listener: listener,
		}
		go func() {
			err := tcpSrv.ActivateAndServe()
			if err != nil {
				log.Fatal(err)
			}
		}()
		servers = append(servers, &tcpSrv)
	}

	return servers, nil
}

func listenAndServeAll(cfg *config.Config, ntsnet *ntsnet.Ntsnet) ([]*dns.Server, error) {
	handler := Handler{
		ntsnet: ntsnet,
	}

	var err error
	handler.dnsConfig, err = dns.ClientConfigFromFile(resolveconfPath)
	if err != nil {
		slog.Error("Reading resolveconf, using fallback", "err", err)
		handler.dnsConfig = &dns.ClientConfig{
			Port: "53",
		}
	}
	handler.dnsConfig.Servers = append(handler.dnsConfig.Servers, cfg.BaseForwardFallback...)

	servers, err := Serve(ntsnet.TsnetServer, &handler)
	if err != nil {
		return servers, nil
	}

	return servers, nil
}

func Run(cfg *config.Config, ntsnet *ntsnet.Ntsnet) error {
	servers, err := listenAndServeAll(cfg, ntsnet)
	if err != nil {
		return fmt.Errorf("Listening on tailnet: %s", err)
	}
	slog.Info("Starting namescale")

	serverCtx, serverCtxCancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(serverCtx, 30*time.Second)
		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatalln("Graceful shutdown timed out, Forcing exit")
			}
		}()

		for _, srv := range servers {
			err := srv.ShutdownContext(shutdownCtx)
			if err != nil {
				log.Fatalln(err)
			}
		}

		err := ntsnet.TsnetServer.Close()
		if err != nil {
			log.Fatalln(err)
		}

		shutdownCtxCancel()
		serverCtxCancel()
	}()

	<-serverCtx.Done()
	return nil
}
