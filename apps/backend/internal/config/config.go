package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DatabaseURL        string
	JWTSecret          string
	FrontendOrigin     string
	OpenRouterAPIKey   string
	OpenRouterModel    string
	E2BAPIKey          string
	E2BTemplateAlias   string
	AgentMaxSteps      int
	AgentTimeout       time.Duration
	SandboxTimeout     time.Duration
	CookieSecure       bool
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func Load() (Config, error) {
	cfg := Config{
		Port:               envOr("PORT", "3010"),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:          strings.TrimSpace(os.Getenv("JWT_SECRET")),
		FrontendOrigin:     envOr("FRONTEND_ORIGIN", "http://localhost:3000"),
		OpenRouterAPIKey:   strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		OpenRouterModel:    strings.TrimSpace(os.Getenv("OPENROUTER_MODEL")),
		E2BAPIKey:          strings.TrimSpace(os.Getenv("E2B_API_KEY")),
		E2BTemplateAlias:   envOr("E2B_TEMPLATE_ALIAS", "cutable-react-base"),
		AgentMaxSteps:      envInt("AGENT_MAX_STEPS", 40),
		AgentTimeout:       envDuration("AGENT_TIMEOUT", 12*time.Minute),
		SandboxTimeout:     envDuration("SANDBOX_TIMEOUT", 15*time.Minute),
		CookieSecure:       envBool("COOKIE_SECURE", false),
		GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURL:  envOr("GOOGLE_REDIRECT_URL", "http://localhost:3010/api/auth/google/callback"),
	}

	var missing []string
	for name, value := range map[string]string{
		"DATABASE_URL":       cfg.DatabaseURL,
		"JWT_SECRET":         cfg.JWTSecret,
		"OPENROUTER_API_KEY": cfg.OpenRouterAPIKey,
		"OPENROUTER_MODEL":   cfg.OpenRouterModel,
		"E2B_API_KEY":        cfg.E2BAPIKey,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if cfg.AgentMaxSteps < 1 || cfg.AgentMaxSteps > 100 {
		return Config{}, errors.New("AGENT_MAX_STEPS must be between 1 and 100")
	}
	if (cfg.GoogleClientID == "") != (cfg.GoogleClientSecret == "") {
		return Config{}, errors.New("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be configured together")
	}
	return cfg, nil
}

func (c Config) GoogleAuthEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
