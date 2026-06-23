package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

type Env string

const (
	EnvLocal Env = "local"
	EnvDev   Env = "dev"
	EnvProd  Env = "prod"
)

type Config struct {
	Env            Env            `yaml:"env"`
	HTTPAddr       string         `yaml:"http_addr"`
	AllowedOrigins []string       `yaml:"allowed_origins"`
	Database       DatabaseConfig `yaml:"database"`
	Redis          RedisConfig    `yaml:"redis"`
	MinIO          MinIOConfig    `yaml:"minio"`
	Video          VideoConfig    `yaml:"video"`
	Auth           AuthConfig     `yaml:"auth"`
	Payment        PaymentConfig  `yaml:"payment"`
	Worker         WorkerConfig   `yaml:"worker"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
}

type MinIOConfig struct {
	Endpoint     string `yaml:"endpoint"`
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	UseSSL       bool   `yaml:"use_ssl"`
	AvatarBucket string `yaml:"avatar_bucket"`
	VideoBucket  string `yaml:"video_bucket"`
}

type VideoConfig struct {
	HLSSecret    string `yaml:"hls_secret"`
	VideoBaseURL string `yaml:"video_base_url"`
	APIBaseURL   string `yaml:"api_base_url"`
}

type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"`
}

type PaymentConfig struct {
	MockEnabled      bool   `yaml:"mock_enabled"`
	PublicBaseURL    string `yaml:"public_base_url"`
	DefaultCurrency  string `yaml:"default_currency"`
	StripeSecretKey  string `yaml:"stripe_secret_key"`
	StripeWebhookKey string `yaml:"stripe_webhook_key"`
	StripeSuccessURL string `yaml:"stripe_success_url"`
	StripeCancelURL  string `yaml:"stripe_cancel_url"`
	PayPalClientID   string `yaml:"paypal_client_id"`
	PayPalSecret     string `yaml:"paypal_secret"`
	PayPalWebhookID  string `yaml:"paypal_webhook_id"`
	PayPalBaseURL    string `yaml:"paypal_base_url"`
}

type WorkerConfig struct {
	TranscodeConcurrency  int    `yaml:"transcode_concurrency"`
	TranscodeVideoEncoder string `yaml:"transcode_video_encoder"`
	TranscodeTempDir      string `yaml:"transcode_temp_dir"`
}

var (
	cached  Config
	loadErr error
	once    sync.Once
)

func Load() Config {
	once.Do(func() {
		cached, loadErr = load()
	})
	if loadErr != nil {
		panic(loadErr)
	}
	return cached
}

func load() (Config, error) {
	env := resolveEnv()
	cfg := defaults(env)
	if err := loadYAML(env, &cfg); err != nil {
		return Config{}, err
	}
	applyEnvOverrides(&cfg)
	return cfg, nil
}

func resolveEnv() Env {
	value := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		os.Getenv("APP_ENV"),
		os.Getenv("GO_ENV"),
		os.Getenv("CONFIG_ENV"),
	)))
	switch Env(value) {
	case EnvDev:
		return EnvDev
	case EnvProd:
		return EnvProd
	default:
		return EnvLocal
	}
}

func defaults(env Env) Config {
	cfg := localDefaults()
	cfg.Env = env
	return cfg
}

func loadYAML(env Env, cfg *Config) error {
	path, ok := findConfigFile(env)
	if !ok {
		return nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.Env = env
	return nil
}

func findConfigFile(env Env) (string, bool) {
	name := string(env) + ".yml"
	candidates := []string{
		filepath.Join("config", name),
		filepath.Join("backend", "config", name),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func localDefaults() Config {
	return Config{
		Env:            EnvLocal,
		HTTPAddr:       ":8080",
		AllowedOrigins: []string{"*"},
		Database: DatabaseConfig{
			DSN: "host=localhost port=5432 user=admin_go password=admin_go_password dbname=flutter_admin_go sslmode=disable TimeZone=Asia/Shanghai",
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
		},
		MinIO: MinIOConfig{
			Endpoint:     "localhost:9000",
			AccessKey:    "admin_go",
			SecretKey:    "admin_go_password",
			AvatarBucket: "admin-avatars",
			VideoBucket:  "video",
		},
		Video: VideoConfig{
			HLSSecret:  "dev_secret",
			APIBaseURL: "http://localhost:8080",
		},
		Auth: AuthConfig{
			JWTSecret: "dev_jwt_secret_change_in_prod",
		},
		Payment: PaymentConfig{
			PublicBaseURL:   "http://localhost:8080",
			DefaultCurrency: "USD",
			PayPalBaseURL:   "https://api-m.sandbox.paypal.com",
		},
		Worker: WorkerConfig{
			TranscodeConcurrency:  2,
			TranscodeVideoEncoder: "auto",
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	cfg.HTTPAddr = envOr("HTTP_ADDR", cfg.HTTPAddr)
	if origins := envList("CORS_ALLOWED_ORIGINS"); len(origins) > 0 {
		cfg.AllowedOrigins = origins
	}
	cfg.Database.DSN = envOr("DATABASE_DSN", cfg.Database.DSN)
	cfg.Redis.Addr = envOr("REDIS_ADDR", cfg.Redis.Addr)

	cfg.MinIO.Endpoint = envOr("MINIO_ENDPOINT", cfg.MinIO.Endpoint)
	cfg.MinIO.AccessKey = envOr("MINIO_ACCESS_KEY", cfg.MinIO.AccessKey)
	cfg.MinIO.SecretKey = envOr("MINIO_SECRET_KEY", cfg.MinIO.SecretKey)
	cfg.MinIO.AvatarBucket = envOr("MINIO_AVATAR_BUCKET", cfg.MinIO.AvatarBucket)
	cfg.MinIO.VideoBucket = envOr("MINIO_VIDEO_BUCKET", cfg.MinIO.VideoBucket)
	cfg.MinIO.UseSSL = envBool("MINIO_USE_SSL", cfg.MinIO.UseSSL)

	cfg.Video.HLSSecret = envOr("HLS_SECRET", cfg.Video.HLSSecret)
	cfg.Video.VideoBaseURL = strings.TrimRight(envOr("VIDEO_BASE_URL", cfg.Video.VideoBaseURL), "/")
	cfg.Video.APIBaseURL = envOr("API_BASE_URL", cfg.Video.APIBaseURL)

	cfg.Auth.JWTSecret = envOr("JWT_SECRET", cfg.Auth.JWTSecret)

	cfg.Payment.MockEnabled = envBool("PAYMENT_MOCK", cfg.Payment.MockEnabled)
	cfg.Payment.PublicBaseURL = envOr("APP_PUBLIC_BASE_URL", cfg.Payment.PublicBaseURL)
	cfg.Payment.DefaultCurrency = envOr("PAYMENT_DEFAULT_CURRENCY", cfg.Payment.DefaultCurrency)
	cfg.Payment.StripeSecretKey = envOr("STRIPE_SECRET_KEY", cfg.Payment.StripeSecretKey)
	cfg.Payment.StripeWebhookKey = envOr("STRIPE_WEBHOOK_SECRET", cfg.Payment.StripeWebhookKey)
	cfg.Payment.StripeSuccessURL = envOr("STRIPE_SUCCESS_URL", cfg.Payment.StripeSuccessURL)
	cfg.Payment.StripeCancelURL = envOr("STRIPE_CANCEL_URL", cfg.Payment.StripeCancelURL)
	cfg.Payment.PayPalClientID = envOr("PAYPAL_CLIENT_ID", cfg.Payment.PayPalClientID)
	cfg.Payment.PayPalSecret = envOr("PAYPAL_CLIENT_SECRET", cfg.Payment.PayPalSecret)
	cfg.Payment.PayPalWebhookID = envOr("PAYPAL_WEBHOOK_ID", cfg.Payment.PayPalWebhookID)
	cfg.Payment.PayPalBaseURL = envOr("PAYPAL_BASE_URL", cfg.Payment.PayPalBaseURL)

	cfg.Worker.TranscodeConcurrency = envInt("TRANSCODE_CONCURRENCY", cfg.Worker.TranscodeConcurrency)
	cfg.Worker.TranscodeVideoEncoder = envOr("TRANSCODE_VIDEO_ENCODER", cfg.Worker.TranscodeVideoEncoder)
	cfg.Worker.TranscodeTempDir = envOr("TRANSCODE_TEMP_DIR", cfg.Worker.TranscodeTempDir)
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

// envList parses a comma-separated environment variable into a trimmed slice.
func envList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return strings.EqualFold(value, "true")
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
