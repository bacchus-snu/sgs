package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"github.com/bacchus-snu/sgs/controller"
	"github.com/bacchus-snu/sgs/model/postgres"
	"github.com/bacchus-snu/sgs/pkg/auth"
	"github.com/bacchus-snu/sgs/worker"
)

type Validator interface {
	Bind()
	Validate() error
}

type Config struct {
	Auth       auth.Config       `mapstructure:"auth"`
	Controller controller.Config `mapstructure:"controller"`
	Postgres   postgres.Config   `mapstructure:"postgres"`
	Worker     worker.Config     `mapstructure:"worker"`
}

var _ Validator = (*Config)(nil)

func Load() (*Config, error) {
	cfg := Config{}
	cfg.Bind()

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Bind() {
	c.Auth.Bind()
	c.Controller.Bind()
	c.Postgres.Bind()
	c.Worker.Bind()
}

func (c *Config) Validate() error {
	var err error

	if err1 := c.Auth.Validate(); err1 != nil {
		err = errors.Join(err, fmt.Errorf("auth: %w", err1))
	}
	if err1 := c.Controller.Validate(); err1 != nil {
		err = errors.Join(err, fmt.Errorf("controller: %w", err1))
	}
	if err1 := c.Postgres.Validate(); err1 != nil {
		err = errors.Join(err, fmt.Errorf("postgres: %w", err1))
	}
	if err1 := c.Worker.Validate(); err1 != nil {
		err = errors.Join(err, fmt.Errorf("worker: %w", err1))
	}

	return err
}
