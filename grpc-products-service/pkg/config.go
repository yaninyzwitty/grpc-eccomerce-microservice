package pkg

import (
	"io"
	"log/slog"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   Server `yaml:"server"`
	Database DB     `yaml:"astra"`
}

type Server struct {
	Port int `yaml:"port"`
}

type DB struct {
	Username string `yaml:"username"`
	// Token string 	`yaml:""token`
	Path    string `yaml:"path"`
	Timeout int    `yaml:"timeout"`
}

func (c *Config) LoadConfig(file io.Reader) error {
	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("failed to read file", "error", err)
		return err
	}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		slog.Error("failed to unmarshal yaml", "error", err)
		return err
	}
	return nil
}
