package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Postgres PostgresConfig `yaml:"postgres"`
	Logger   LoggerConfig   `yaml:"logger"`
}

type HTTPConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxOpenConns    int32         `yaml:"max_open_conns"`
	MinIdleConns    int32         `yaml:"min_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type LoggerConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	OutputDir string `yaml:"output_dir"`
	AddSource bool   `yaml:"add_source"`
	Service   string `yaml:"service"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.HTTP.Host == "" {
		cfg.HTTP.Host = "0.0.0.0"
	}
	if cfg.HTTP.Port == 0 {
		cfg.HTTP.Port = 8080
	}
	if cfg.HTTP.ReadTimeout == 0 {
		cfg.HTTP.ReadTimeout = 5 * time.Second
	}
	if cfg.HTTP.WriteTimeout == 0 {
		cfg.HTTP.WriteTimeout = 10 * time.Second
	}
	if cfg.HTTP.ShutdownTimeout == 0 {
		cfg.HTTP.ShutdownTimeout = 10 * time.Second
	}
	if cfg.Postgres.Port == 0 {
		cfg.Postgres.Port = 5432
	}
	if cfg.Postgres.SSLMode == "" {
		cfg.Postgres.SSLMode = "disable"
	}
	if cfg.Postgres.MaxOpenConns == 0 {
		cfg.Postgres.MaxOpenConns = 10
	}
	if cfg.Postgres.MinIdleConns == 0 {
		cfg.Postgres.MinIdleConns = 2
	}
	if cfg.Postgres.ConnMaxLifetime == 0 {
		cfg.Postgres.ConnMaxLifetime = time.Hour
	}
	if cfg.Logger.Level == "" {
		cfg.Logger.Level = "info"
	}
	if cfg.Logger.Format == "" {
		cfg.Logger.Format = "json"
	}
	if cfg.Logger.OutputDir == "" {
		cfg.Logger.OutputDir = "./var/log/subscriptions"
	}
	if cfg.Logger.Service == "" {
		cfg.Logger.Service = "subscriptions-service"
	}

	applyEnvOverrides(&cfg)

	return cfg, nil
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d pool_max_conn_lifetime=%s", c.Host, c.Port, c.Database, c.User, c.Password, c.SSLMode, c.MaxOpenConns, c.MinIdleConns, c.ConnMaxLifetime)
}

func applyEnvOverrides(cfg *Config) {
	cfg.HTTP.Host = envString("APP_HTTP_HOST", cfg.HTTP.Host)
	cfg.HTTP.Port = envInt("APP_HTTP_PORT", cfg.HTTP.Port)
	cfg.HTTP.ReadTimeout = envDuration("APP_HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout)
	cfg.HTTP.WriteTimeout = envDuration("APP_HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout)
	cfg.HTTP.ShutdownTimeout = envDuration("APP_HTTP_SHUTDOWN_TIMEOUT", cfg.HTTP.ShutdownTimeout)

	cfg.Postgres.Host = envString("APP_POSTGRES_HOST", cfg.Postgres.Host)
	cfg.Postgres.Port = envInt("APP_POSTGRES_PORT", cfg.Postgres.Port)
	cfg.Postgres.Database = envString("APP_POSTGRES_DATABASE", cfg.Postgres.Database)
	cfg.Postgres.User = envString("APP_POSTGRES_USER", cfg.Postgres.User)
	cfg.Postgres.Password = envString("APP_POSTGRES_PASSWORD", cfg.Postgres.Password)
	cfg.Postgres.SSLMode = envString("APP_POSTGRES_SSL_MODE", cfg.Postgres.SSLMode)
	cfg.Postgres.MaxOpenConns = int32(envInt("APP_POSTGRES_MAX_OPEN_CONNS", int(cfg.Postgres.MaxOpenConns)))
	cfg.Postgres.MinIdleConns = int32(envInt("APP_POSTGRES_MIN_IDLE_CONNS", int(cfg.Postgres.MinIdleConns)))
	cfg.Postgres.ConnMaxLifetime = envDuration("APP_POSTGRES_CONN_MAX_LIFETIME", cfg.Postgres.ConnMaxLifetime)

	cfg.Logger.Level = envString("APP_LOGGER_LEVEL", cfg.Logger.Level)
	cfg.Logger.Format = envString("APP_LOGGER_FORMAT", cfg.Logger.Format)
	cfg.Logger.OutputDir = envString("APP_LOGGER_OUTPUT_DIR", cfg.Logger.OutputDir)
	cfg.Logger.AddSource = envBool("APP_LOGGER_ADD_SOURCE", cfg.Logger.AddSource)
	cfg.Logger.Service = envString("APP_LOGGER_SERVICE", cfg.Logger.Service)
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
