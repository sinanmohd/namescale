package dns

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"sinanmohd.com/namescale/internal/config"
)

const (
	RESOLVECONF_PATH = "/etc/resolv.conf"
	HEADSCALE_NS     = "100.100.100.100"
)

type Handler struct {
	dnsConfig      *dns.ClientConfig
	baseDomainFqdn string
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
		resp, _, err = client.Exchange(req, net.JoinHostPort(upstream, handler.dnsConfig.Port))
		if err == nil {
			break
		}

		slog.Error("Root NS resolving", "err", err)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
	}

	w.WriteMsg(resp)
}

func (handler *Handler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	if req.Opcode != dns.OpcodeQuery {
		slog.Error("Ignoring non-query request", "name", req.Question[0].Name, "opcode", req.Opcode)
		w.WriteMsg(req.SetRcode(req, dns.RcodeServerFailure))
		return
	}

	client := new(dns.Client)
	var qustionNames []string
	for i := range req.Question {
		// pass the base domain to root ns
		if req.Question[i].Name == handler.baseDomainFqdn {
			handler.ServeFromRootNS(client, w, req)
			return
		}

		// handle the rest (wild card)
		hostFqdn, err := hostFqdnFromWildQustion(req.Question[i].Name, handler.baseDomainFqdn)
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
		slog.Error("Unexpected dns msg length", "qustionLen", qustionLen, "answerLen", answerLen, "respQustionLen", respQustionLen)
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

func listenAndServeTransport(addr, transport string, handler *Handler) *dns.Server {
	srv := dns.Server{
		Net:       transport,
		Addr:      addr,
		ReusePort: true,
		Handler:   handler,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	return &srv
}

func listenAndServeAll(cfg *config.Config) ([]*dns.Server, error) {
	var servers []*dns.Server

	handler := Handler{
		baseDomainFqdn: cfg.BaseDomain,
	}

	var err error
	handler.dnsConfig, err = dns.ClientConfigFromFile(RESOLVECONF_PATH)
	if err != nil {
		return nil, fmt.Errorf("Reading %s: %s", RESOLVECONF_PATH, err)
	}

	srv := listenAndServeTransport(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "tcp", &handler)
	servers = append(servers, srv)
	srv = listenAndServeTransport(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "udp", &handler)
	servers = append(servers, srv)
	return servers, nil
}

func Run(cfg *config.Config) error {
	servers, err := listenAndServeAll(cfg)
	if err != nil {
		return fmt.Errorf("Listening on all transport: %s", err)
	}
	slog.Info("Server listening for requests", "host", cfg.Host, "port", cfg.Port)

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

		shutdownCtxCancel()
		serverCtxCancel()
	}()

	<-serverCtx.Done()
	return nil
}
