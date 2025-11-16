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
	RESOLVECONF_PATH = "/etc/resolv.conf"
	HEADSCALE_NS     = "100.100.100.100"
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

func (handler *Handler) ServeFromRootNS(client *dns.Client, w dns.ResponseWriter, req *dns.Msg) {
	var resp *dns.Msg
	var err error

	for _, upstream := range handler.dnsConfig.Servers {
		if upstream == HEADSCALE_NS {
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

	client := new(dns.Client)
	var qustionNames []string
	for i := range req.Question {
		// pass the base domain to root ns
		if req.Question[i].Name == baseDomain {
			handler.ServeFromRootNS(client, w, req)
			return
		}

		// handle the rest (wild card)
		hostFqdn, err := hostFqdnFromWildQustion(req.Question[i].Name, baseDomain)
		if err != nil {
			slog.Error("Getting hostFqdn", "err", err)
			w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
			return
		}

		// either MagicDNS is disabled, or non-existent host
		if req.Question[i].Name == hostFqdn {
			w.WriteMsg(req.SetRcode(req, dns.RcodeNameError))
			return
		}

		qustionNames = append(qustionNames, req.Question[i].Name)
		req.Question[i].Name = hostFqdn
	}

	resp, _, err := client.Exchange(req, net.JoinHostPort(HEADSCALE_NS, handler.dnsConfig.Port))
	if err != nil {
		slog.Error("Headscale NS resolving", "err", err)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
	}

	qustionLen := len(qustionNames)
	respQustionLen := len(resp.Question)
	answerLen := len(resp.Answer)
	if qustionLen != respQustionLen || qustionLen != answerLen {
		slog.Error(
			"Unexpected dns msg length",
			"qustionLen",
			qustionLen,
			"answerLen",
			answerLen,
			"respQustionLen",
			respQustionLen,
		)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
		return
	}
	for i := range resp.Question {
		resp.Question[i].Name = qustionNames[i]
	}
	for i := range resp.Answer {
		header := resp.Answer[i].Header()
		header.Name = qustionNames[i]
	}

	w.WriteMsg(resp)
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
	handler.dnsConfig, err = dns.ClientConfigFromFile(RESOLVECONF_PATH)
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
