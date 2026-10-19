package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents runtime configuration parameters for the Recourse microservice.
type Config struct {
	Port                string        `json:"port"`
	ServiceName         string        `json:"service_name"`
	DatabaseURL         string        `json:"database_url"`
	DBMaxConns          int32         `json:"db_max_conns"`
	DBMinConns          int32         `json:"db_min_conns"`
	DBMaxConnLifetime   time.Duration `json:"db_max_conn_lifetime"`
	DBMaxConnIdleTime   time.Duration `json:"db_max_conn_idle_time"`
	DBHealthCheckPeriod time.Duration `json:"db_health_check_period"`
	JWTSecret           string        `json:"-"` // Redacted from logs
	JWTExpiration       time.Duration `json:"jwt_expiration"`
	RequestTimeout      time.Duration `json:"request_timeout"`
	LogLevel            string        `json:"log_level"`
}

// LoadFromEnv loads configuration parameters with sensible production defaults.
func LoadFromEnv() *Config {
	port := getEnv("PORT", "8083")
	if port[0] != ':' && port != "" {
		port = ":" + port
	}

	maxConns := int32(getEnvInt("DB_MAX_CONNS", 25))
	minConns := int32(getEnvInt("DB_MIN_CONNS", 5))
	jwtHours := getEnvInt("JWT_EXPIRATION_HOURS", 24)
	timeoutSec := getEnvInt("REQUEST_TIMEOUT_SEC", 5)

	return &Config{
		Port:                port,
		ServiceName:         getEnv("SERVICE_NAME", "recourse-backend"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/recourse_db?sslmode=disable"),
		DBMaxConns:          maxConns,
		DBMinConns:          minConns,
		DBMaxConnLifetime:   1 * time.Hour,
		DBMaxConnIdleTime:   30 * time.Minute,
		DBHealthCheckPeriod: 1 * time.Minute,
		JWTSecret:           getEnv("JWT_SECRET", "recourse-enterprise-jwt-signing-secret-2026-key"),
		JWTExpiration:       time.Duration(jwtHours) * time.Hour,
		RequestTimeout:      time.Duration(timeoutSec) * time.Second,
		LogLevel:            getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
