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

// ThinkingStyle describes how a model supports thinking/reasoning.
type ThinkingStyle string

const (
	ThinkingStyleNone     ThinkingStyle = ""         // model does not support thinking
	ThinkingStyleAdaptive ThinkingStyle = "adaptive" // Anthropic adaptive thinking (opus, mythos)
	ThinkingStyleBudget   ThinkingStyle = "budget"   // Anthropic enabled thinking with explicit budget_tokens
	ThinkingStyleEffort   ThinkingStyle = "effort"   // OpenAI reasoning_effort (o-series)
)

// Model maps a friendly model alias to provider-specific details.
type Model struct {
	Name            string        // friendly alias for a model (e.g. "sonnet-4.6").
	OLTPProvider    OLTPProvider  // The LLM provider (anthropic, openai, mistral, _other) following OpenTelemetry GenAI semantic conventions
	APIClient       APIClient     // The SDK client to use for this model (e.g. OpenAI, Anthropic)
	APIKeyName      string        // Name of the environment variable for the API key
	URL             string        // Optional base URL for API-compatible providers
	APIModelID      string        // The model ID to use in the API request (e.g. "claude-sonnet-4-5")
	ThinkingStyle   ThinkingStyle // how the model supports thinking/reasoning (empty = not supported)
	MaxOutputTokens int64         // maximum output tokens for the model
}

// registry holds all supported models.
var registry = []Model{
	{Name: "haiku-4.5", OLTPProvider: OLTPProviderAnthropic, APIClient: APIClientAnthropic, APIKeyName: "ANTHROPIC_API_KEY", APIModelID: "claude-haiku-4-5", ThinkingStyle: ThinkingStyleBudget, MaxOutputTokens: 8192},
	{Name: "sonnet-4.6", OLTPProvider: OLTPProviderAnthropic, APIClient: APIClientAnthropic, APIKeyName: "ANTHROPIC_API_KEY", APIModelID: "claude-sonnet-4-5", ThinkingStyle: ThinkingStyleBudget, MaxOutputTokens: 16384},
	{Name: "opus", OLTPProvider: OLTPProviderAnthropic, APIClient: APIClientAnthropic, APIKeyName: "ANTHROPIC_API_KEY", APIModelID: "claude-opus-4", ThinkingStyle: ThinkingStyleAdaptive, MaxOutputTokens: 16384},
	{Name: "o4-mini", OLTPProvider: OLTPProviderOpenAI, APIClient: APIClientOpenAI, APIKeyName: "OPENAI_API_KEY", APIModelID: "o4-mini", ThinkingStyle: ThinkingStyleEffort, MaxOutputTokens: 16384},
	{Name: "gpt-5.4", OLTPProvider: OLTPProviderOpenAI, APIClient: APIClientOpenAI, APIKeyName: "OPENAI_API_KEY", APIModelID: "gpt-4o", MaxOutputTokens: 16384},
	{Name: "mistral-small", OLTPProvider: OLTPProviderMistral, APIClient: APIClientOpenAI, APIKeyName: "MISTRAL_API_KEY", URL: "https://api.mistral.ai/v1", APIModelID: "mistral-small-2506", MaxOutputTokens: 8192},
	{Name: "agent", OLTPProvider: OLTPProviderPoolside, APIClient: APIClientOpenAI, APIKeyName: "POOLSIDE_API_KEY", URL: "https://poolside.srvgpu-poolside02.bsfr.bs.fr.myatos.net/openai/v1", APIModelID: "agent", MaxOutputTokens: 4096},
}

// Lookup returns the model details for a given alias.
func Lookup(modelID string) (Model, error) {
	for _, descriptor := range registry {
		if descriptor.Name == modelID {
			return descriptor, nil
		}
	}

	return Model{}, fmt.Errorf("unknown model %q", modelID)
}

// SupportedModels returns all registered model aliases.
func SupportedModels() []string {
	models := make([]string, 0, len(registry))
	for _, descriptor := range registry {
		models = append(models, descriptor.Name)
	}
	return models
}
