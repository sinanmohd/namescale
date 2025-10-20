package config

import (
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/miekg/dns"
)

type Config struct {
	Host                string   `toml:"host"`
	Port                uint     `toml:"port"`
	BaseDomain          string   `toml:"base_domain"`
	BaseForwardFallback []string `toml:"base_forward_fallback"`
}

func New() (*Config, error) {
	var configPath string
	defaultConfigPath := "/etc/namescale.toml"
	if value, ok := os.LookupEnv("NAMESCALE_CONFIG"); ok {
		configPath = value
	} else {
		configPath = defaultConfigPath
	}

	config := Config{
		Host:                "[::]",
		Port:                53,
		BaseDomain:          "ts.net.",
		BaseForwardFallback: []string{"1.1.1.1", "8.8.8.8"},
	}

	_, err := os.Stat(configPath)
	if err != nil {
		if (configPath == defaultConfigPath && !errors.Is(err, os.ErrNotExist)) ||
			configPath != defaultConfigPath {
			slog.Error("Error reading config", "err", err)
			return nil, err
		}
	}

	if _, err := os.Stat(configPath); err == nil {
		_, err := toml.DecodeFile(configPath, &config)
		if err != nil {
			slog.Error("Error decoding TOML", "err", err)
			return nil, err
		}
	}

	flag.StringVar(&config.Host, "host", config.Host, "Bind host")
	flag.UintVar(&config.Port, "port", config.Port, "Bind port")
	flag.StringVar(
		&config.BaseDomain,
		"base-domain",
		config.BaseDomain,
		"Base domain (dns.base_domain in headscale)",
	)
	flag.Parse()

	config.BaseDomain = dns.Fqdn(config.BaseDomain)

	return &config, nil
}
