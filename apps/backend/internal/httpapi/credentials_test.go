package httpapi

import "testing"

func TestProviderCredentialsValidation(t *testing.T) {
	t.Run("empty uses demo allowance", func(t *testing.T) {
		if err := (providerCredentials{}).validate(); err != nil {
			t.Fatalf("empty credentials should be accepted for demo routing: %v", err)
		}
	})

	t.Run("requires both keys", func(t *testing.T) {
		err := (providerCredentials{OpenRouterAPIKey: "sk-or-test-key-long-enough"}).validate()
		if err == nil {
			t.Fatal("expected partial provider credentials to fail")
		}
	})

	t.Run("accepts bounded keys", func(t *testing.T) {
		err := (providerCredentials{
			OpenRouterAPIKey: "sk-or-test-key-long-enough",
			E2BAPIKey:        "e2b-test-key-long-enough",
		}).validate()
		if err != nil {
			t.Fatalf("complete credentials rejected: %v", err)
		}
	})
}
