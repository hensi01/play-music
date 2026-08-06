package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// S3Config holds the MinIO/S3 connection settings, sourced from the
// ND_MUSICFOLDER URL (preferred) with ND_S3_* and MINIO_* fallbacks.
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
	AdminUsername string
	AdminPassword string

	S3 S3Config

	RedisEnabled bool
	RedisURL     string

	CDNEnabled      bool
	CDNBaseURL      string
	CDNTokenKey     string
	CDNTokenTTL     time.Duration
	CDNAdvancedAuth bool
	CDNPathPrefix   string

	DatabaseURL string

	FfmpegPath string

	Port           int
	Address        string
	LogLevel       string
	ScannerSchedule string

	TranscodingCacheSize int64
	ImageCacheSize       int64

	EnableInsights bool

	MusicFolder string
}

func getenv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
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

// Load reads the configuration from the environment.
func Load() *Config {
	musicFolder := getenv("ND_MUSICFOLDER")

	s3 := s3FromMusicFolder(musicFolder)
	if s3.Endpoint == "" {
		s3.Endpoint = getenv("ND_S3_ENDPOINT")
	}
	if s3.Endpoint == "" {
		s3.Endpoint = getenv("MINIO_ENDPOINT")
	}
	if s3.AccessKey == "" {
		s3.AccessKey = getenv("ND_S3_ACCESSKEY")
	}
	if s3.AccessKey == "" {
		s3.AccessKey = getenv("MINIO_ACCESS_KEY")
	}
	if s3.SecretKey == "" {
		s3.SecretKey = getenv("ND_S3_SECRETKEY")
	}
	if s3.SecretKey == "" {
		s3.SecretKey = getenv("MINIO_SECRET_KEY")
	}
	if s3.Bucket == "" {
		s3.Bucket = getenv("ND_S3_BUCKET")
	}
	if s3.Bucket == "" {
		s3.Bucket = getenv("MINIO_BUCKET")
	}
	if s3.Region == "" {
		s3.Region = getenv("ND_S3_REGION")
	}
	if s3.Region == "" {
		s3.Region = getenv("MINIO_REGION")
	}
	if !s3.Secure && getenv("ND_S3_SECURE") != "" {
		s3.Secure = getenvBool("ND_S3_SECURE", true)
	}
	if getenv("MINIO_USE_SSL") != "" {
		s3.Secure = getenvBool("MINIO_USE_SSL", true)
	}

	ttl, _ := time.ParseDuration(getenv("ND_CDN_TOKENTTL"))
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &Config{
		AdminUsername: getenv("ND_ADMINUSERNAME"),
		AdminPassword: getenv("ND_ADMINPASSWORD"),

		S3: s3,

		RedisEnabled: getenvBool("ND_REDIS_ENABLED", false),
		RedisURL:     getenv("ND_REDIS_URL"),

		CDNEnabled:      getenvBool("ND_CDN_ENABLED", false),
		CDNBaseURL:      getenv("ND_CDN_BASEURL"),
		CDNTokenKey:     getenv("ND_CDN_TOKENAUTHKEY"),
		CDNTokenTTL:     ttl,
		CDNAdvancedAuth: getenvBool("ND_CDN_ADVANCEDAUTH", false),
		CDNPathPrefix:   getenv("ND_CDN_PATH_PREFIX"),

		DatabaseURL: getenv("DATABASE_URL"),

		FfmpegPath: getenv("ND_FFMPEGPATH"),

		Port:             getenvInt("ND_PORT", 4533),
		Address:          getenv("ND_ADDRESS"),
		LogLevel:         getenv("ND_LOGLEVEL"),
		ScannerSchedule:  getenv("ND_SCANNER_SCHEDULE"),

		TranscodingCacheSize: parseSize(getenv("ND_TRANSCODINGCACHESIZE")),
		ImageCacheSize:       parseSize(getenv("ND_IMAGECACHESIZE")),

		EnableInsights: getenvBool("ND_ENABLEINSIGHTSCOLLECTOR", false),

		MusicFolder: musicFolder,
	}
}

// ListenAddr returns the listen address (host:port).
func (c *Config) ListenAddr() string {
	host := c.Address
	if host == "" {
		host = "0.0.0.0"
	}
	return host + ":" + strconv.Itoa(c.Port)
}
