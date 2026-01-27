package email

import (
	"errors"

	"github.com/spf13/viper"
)

type Config struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

func (c *Config) Bind() {
	viper.BindEnv("email.host", "SGS_EMAIL_HOST")
	viper.BindEnv("email.port", "SGS_EMAIL_PORT")
	viper.BindEnv("email.username", "SGS_EMAIL_USERNAME")
	viper.BindEnv("email.password", "SGS_EMAIL_PASSWORD")
	viper.BindEnv("email.from", "SGS_EMAIL_FROM")

	// Defaults
	viper.SetDefault("email.host", "smtp.gmail.com")
	viper.SetDefault("email.port", 587)
}

func (c *Config) Validate() error {
	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("email.host is required"))
	}
	if c.Port == 0 {
		errs = append(errs, errors.New("email.port is required"))
	}
	if c.Username == "" {
		errs = append(errs, errors.New("email.username is required"))
	}
	if c.Password == "" {
		errs = append(errs, errors.New("email.password is required"))
	}
	if c.From == "" {
		errs = append(errs, errors.New("email.from is required"))
	}
	return errors.Join(errs...)
}
