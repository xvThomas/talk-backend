package domain

import "fmt"

// APIClient identifies the specific API client to use for a model (the SDK)
type APIClient string

const (
	APIClientOpenAI    APIClient = "openai"
	APIClientAnthropic APIClient = "anthropic"
)

// OLTPProvider identifies the LLM provider backend.
type OLTPProvider string

/*
OLTP GenAI semantic conventions for gen_ai.system (https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/):
openai	OpenAI
anthropic	Anthropic
aws.bedrock	AWS Bedrock
az.ai.inference	Azure AI Inference
az.ai.openai	Azure OpenAI
google_vertexai	Google Vertex AI
google_generativeai	Google Gemini
cohere	Cohere
mistral_ai	Mistral AI
perplexity	Perplexity
xai	xAI
deepseek	DeepSeek
groq	Groq
ibm.watsonx_ai	IBM Watsonx
_other	Other provider (use with gen_ai.system_description)
*/
const (
	OLTPProviderAnthropic OLTPProvider = "anthropic"
	OLTPProviderOpenAI    OLTPProvider = "openai"
	OLTPProviderMistral   OLTPProvider = "mistral_ai"
	OLTPProviderPoolside  OLTPProvider = "_other"
)

// Model is a friendly alias for a model (e.g. "sonnet-4.6").
type Model string

// ModelDescriptor maps a friendly Model alias to provider-specific details.
type ModelDescriptor struct {
	OLTPProvider OLTPProvider
	APIClient    APIClient
	APIKeyName   string  // Name of the environment variable for the API key
	URL          string // Optional base URL for API-compatible providers
	APIModelID   string
}

// registry holds all supported model aliases.
var registry = map[Model]ModelDescriptor{
	"haiku-4.5":     {OLTPProvider: OLTPProviderAnthropic, APIClient: APIClientAnthropic, APIKeyName: "ANTHROPIC_API_KEY", URL: "", APIModelID: "claude-haiku-4-5"},
	"sonnet-4.6":    {OLTPProvider: OLTPProviderAnthropic, APIClient: APIClientAnthropic, APIKeyName: "ANTHROPIC_API_KEY", URL: "", APIModelID: "claude-sonnet-4-5"},
	"gpt-5.4":       {OLTPProvider: OLTPProviderOpenAI, APIClient: APIClientOpenAI, APIKeyName: "OPENAI_API_KEY", URL: "", APIModelID: "gpt-4o"},
	"mistral-small": {OLTPProvider: OLTPProviderMistral, APIClient: APIClientOpenAI, APIKeyName: "MISTRAL_API_KEY", URL: "https://api.mistral.ai/v1", APIModelID: "mistral-small-2506"},
	"agent":         {OLTPProvider: OLTPProviderPoolside, APIClient: APIClientOpenAI, APIKeyName: "POOLSIDE_API_KEY", URL: "https://poolside.srvgpu-poolside02.bsfr.bs.fr.myatos.net/openai/v1", APIModelID: "agent"},
}

// Lookup returns the ModelDescriptor for a given alias.
func Lookup(m Model) (ModelDescriptor, error) {
	d, ok := registry[m]
	if !ok {
		return ModelDescriptor{}, fmt.Errorf("unknown model %q", m)
	}
	return d, nil
}

// SupportedModels returns all registered model aliases.
func SupportedModels() []Model {
	models := make([]Model, 0, len(registry))
	for m := range registry {
		models = append(models, m)
	}
	return models
}
