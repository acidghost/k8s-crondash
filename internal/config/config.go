package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/alecthomas/kong"
)

type Config struct {
	ListenAddr      string `default:":3000" help:"HTTP listen address"`
	Namespace       string `help:"Namespace to watch (empty = all)"`
	RefreshInterval int    `default:"5" help:"HTMX poll interval in seconds"`
	JobHistoryLimit int    `default:"5" help:"Max child jobs per cronjob"`
	AuthUsername    string `required:"" help:"Basic auth username"`
	AuthPassword    string `required:"" help:"Basic auth password"`
}

func Load() (*Config, error) {
	var cfg Config
	parser, err := kong.New(&cfg, kong.DefaultEnvars("CRONDASH"))
	if err != nil {
		return nil, fmt.Errorf("failed to create config parser: %w", err)
	}
	if _, err := parser.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate(_ *kong.Context) error {
	var errs []error
	if _, err := net.ResolveTCPAddr("tcp", c.ListenAddr); err != nil {
		errs = append(errs, fmt.Errorf("listen-addr %q is not a valid host:port: %w", c.ListenAddr, err))
	}
	if c.RefreshInterval < 1 {
		errs = append(errs, fmt.Errorf("refresh-interval must be >= 1, got %d", c.RefreshInterval))
	}
	if c.JobHistoryLimit < 1 {
		errs = append(errs, fmt.Errorf("job-history-limit must be >= 1, got %d", c.JobHistoryLimit))
	}
	return errors.Join(errs...)
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{ListenAddr:%q Namespace:%q RefreshInterval:%d JobHistoryLimit:%d AuthUsername:%q}",
		c.ListenAddr, c.Namespace, c.RefreshInterval, c.JobHistoryLimit, c.AuthUsername)
}
