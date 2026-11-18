package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	Secure    bool
}

// Config is the runtime configuration, loaded exclusively from the
// environment variables defined in .env (ND_* prefix + DATABASE_URL).
type Config struct {
	PushProvider, RelayBaseURL, RelayAppID, RelayAppSecret, RelayVapidPublicKey string
	AdminUsername                                                               string
	AdminPassword                                                               string

	S3 S3Config

	RedisEnabled bool
	RedisURL     string

	StreamURLTTL time.Duration

	DatabaseURL string

	FfmpegPath string

	// FrontendBaseURL é a URL base da API vista pelo navegador
	// (injetada em index.html -> window.__APP_CONFIG__.baseURL).
	// Vazia = mesmo domínio do frontend.
	FrontendBaseURL string

	Port                 int
	Address              string
	LogLevel             string
	ScannerSchedule      string
	ScannerDeleteMissing bool

	TranscodingCacheSize int64
	ImageCacheSize       int64
}

func getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

// firstSet returns the first non-empty value.
func firstSet(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// firstBool parses the first environment variable that is set; def is used when
// none of them is.
func firstBool(def bool, keys ...string) bool {
	for _, k := range keys {
		if getenv(k) != "" {
			return getenvBool(k, def)
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvInt(key string, def int) int {
	v := getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// parseSize parses sizes like "500MiB", "100MB", "1GiB" or plain bytes.
func parseSize(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	lower := strings.ToLower(v)
	mult := int64(1)
	switch {
	case strings.HasSuffix(lower, "kib"):
		mult, lower = 1<<10, strings.TrimSuffix(lower, "kib")
	case strings.HasSuffix(lower, "mib"):
		mult, lower = 1<<20, strings.TrimSuffix(lower, "mib")
	case strings.HasSuffix(lower, "gib"):
		mult, lower = 1<<30, strings.TrimSuffix(lower, "gib")
	case strings.HasSuffix(lower, "kb"):
		mult, lower = 1000, strings.TrimSuffix(lower, "kb")
	case strings.HasSuffix(lower, "mb"):
		mult, lower = 1000_000, strings.TrimSuffix(lower, "mb")
	case strings.HasSuffix(lower, "gb"):
		mult, lower = 1000_000_000, strings.TrimSuffix(lower, "gb")
	case strings.HasSuffix(lower, "b"):
		lower = strings.TrimSuffix(lower, "b")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(lower), 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}

func s3FromMusicFolder(musicFolder string) S3Config {
	cfg := S3Config{}
	u, err := url.Parse(musicFolder)
	if err == nil && u.Scheme == "s3" && u.Host != "" {
		cfg.Bucket = u.Host
		q := u.Query()
		cfg.Endpoint = q.Get("endpoint")
		cfg.AccessKey = q.Get("accessKey")
		cfg.SecretKey = q.Get("secretKey")
		cfg.Region = q.Get("region")
		cfg.Secure = q.Get("secure") != "false" && q.Get("secure") != "0"
	}
	return cfg
}

func Load() *Config {
	musicFolder := getenv("ND_MUSICFOLDER")

	s3 := s3FromMusicFolder(musicFolder)
	// Each field falls back in order: ND_MUSICFOLDER, the ND_S3_* name, then
	// the MINIO_* name that older deployments still set.
	s3.Endpoint = firstSet(s3.Endpoint, getenv("ND_S3_ENDPOINT"), getenv("MINIO_ENDPOINT"))
	s3.AccessKey = firstSet(s3.AccessKey, getenv("ND_S3_ACCESSKEY"), getenv("MINIO_ACCESS_KEY"))
	s3.SecretKey = firstSet(s3.SecretKey, getenv("ND_S3_SECRETKEY"), getenv("MINIO_SECRET_KEY"))
	s3.Bucket = firstSet(s3.Bucket, getenv("ND_S3_BUCKET"), getenv("MINIO_BUCKET"))
	s3.Region = firstSet(s3.Region, getenv("ND_S3_REGION"), getenv("MINIO_REGION"))
	s3.Secure = firstBool(s3.Secure, "ND_S3_SECURE", "MINIO_USE_SSL")

	ttl, _ := time.ParseDuration(getenv("ND_STREAM_URL_TTL"))
	if ttl < time.Second || ttl > 7*24*time.Hour {
		ttl = time.Hour
	}

	return &Config{
		PushProvider: getenv("PUSH_PROVIDER"), RelayBaseURL: getenv("RELAY_BASE_URL"), RelayAppID: getenv("RELAY_APP_ID"), RelayAppSecret: getenv("RELAY_APP_SECRET"), RelayVapidPublicKey: getenv("RELAY_VAPID_PUBLIC_KEY"),
		AdminUsername: getenv("ND_ADMINUSERNAME"),
		AdminPassword: getenv("ND_ADMINPASSWORD"),

		S3: s3,

		RedisEnabled: getenvBool("ND_REDIS_ENABLED", false),
		RedisURL:     getenv("ND_REDIS_URL"),

		StreamURLTTL: ttl,

		DatabaseURL: getenv("DATABASE_URL"),

		FfmpegPath: getenv("ND_FFMPEGPATH"),

		FrontendBaseURL: getenv("ND_FRONTEND_BASEURL"),

		Port:                 getenvInt("ND_PORT", 4533),
		Address:              getenv("ND_ADDRESS"),
		LogLevel:             getenv("ND_LOGLEVEL"),
		ScannerSchedule:      getenv("ND_SCANNER_SCHEDULE"),
		ScannerDeleteMissing: getenvBool("ND_SCANNER_DELETE_MISSING", false),

		TranscodingCacheSize: parseSize(getenv("ND_TRANSCODINGCACHESIZE")),
		ImageCacheSize:       parseSize(getenv("ND_IMAGECACHESIZE")),
	}
}

func (c *Config) ListenAddr() string {
	host := c.Address
	if host == "" {
		host = "0.0.0.0"
	}
	return host + ":" + strconv.Itoa(c.Port)
}
