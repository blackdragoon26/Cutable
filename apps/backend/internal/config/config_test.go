package config

import "testing"

func TestLoadRequiresStrongConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/cutable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter")
	t.Setenv("OPENROUTER_MODEL", "test/model")
	t.Setenv("E2B_API_KEY", "test-e2b")
	t.Setenv("AGENT_MAX_STEPS", "12")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "3010" || cfg.AgentMaxSteps != 12 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.GoogleAuthEnabled() {
		t.Fatal("Google auth unexpectedly enabled")
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/cutable")
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter")
	t.Setenv("OPENROUTER_MODEL", "test/model")
	t.Setenv("E2B_API_KEY", "test-e2b")

	if _, err := Load(); err == nil {
		t.Fatal("Load() unexpectedly accepted a short JWT secret")
	}
}

func TestLoadGoogleConfigurationMustBeComplete(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/cutable")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter")
	t.Setenv("OPENROUTER_MODEL", "test/model")
	t.Setenv("E2B_API_KEY", "test-e2b")
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted incomplete Google OAuth configuration")
	}
}
