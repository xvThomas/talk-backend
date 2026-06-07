package domain

import (
	"strings"
	"testing"
)

func TestLookup_KnownModel(t *testing.T) {
	d, err := Lookup("sonnet-4.6")
	if err != nil {
		t.Fatalf("Lookup returned unexpected error: %v", err)
	}

	if d.Name != "sonnet-4.6" {
		t.Fatalf("expected ModelID %q, got %q", "sonnet-4.6", d.Name)
	}
	if d.OLTPProvider != OLTPProviderAnthropic {
		t.Fatalf("expected OLTPProvider %q, got %q", OLTPProviderAnthropic, d.OLTPProvider)
	}
	if d.APIClient != APIClientAnthropic {
		t.Fatalf("expected APIClient %q, got %q", APIClientAnthropic, d.APIClient)
	}
	if d.APIModelID != "claude-sonnet-4-5" {
		t.Fatalf("expected APIModelID %q, got %q", "claude-sonnet-4-5", d.APIModelID)
	}
}

func TestLookup_UnknownModel(t *testing.T) {
	_, err := Lookup("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}
	if !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("expected error to contain %q, got %q", "unknown model", err.Error())
	}
}

func TestSupportedModels(t *testing.T) {
	models := SupportedModels()

	if len(models) != len(registry) {
		t.Fatalf("expected %d models, got %d", len(registry), len(models))
	}

	seen := make(map[string]struct{}, len(models))
	for i, model := range models {
		if model == "" {
			t.Fatalf("model at index %d is empty", i)
		}
		if _, ok := seen[model]; ok {
			t.Fatalf("duplicate model %q in SupportedModels", model)
		}
		seen[model] = struct{}{}
	}

	for _, expected := range registry {
		if _, ok := seen[expected.Name]; !ok {
			t.Fatalf("missing model %q in SupportedModels", expected.Name)
		}
	}
}
