module github.com/xvThomas/LLMClientWrapper/talk

go 1.25.0

require (
	github.com/anthropics/anthropic-sdk-go v1.27.1
	github.com/joho/godotenv v1.5.1
	github.com/openai/openai-go v1.12.0
	github.com/spf13/cobra v1.10.2
	github.com/swaggest/jsonschema-go v0.3.79
	github.com/xvThomas/LLMClientWrapper/talk-libs v0.0.0
	golang.org/x/term v0.41.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/swaggest/refl v1.4.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

replace github.com/xvThomas/LLMClientWrapper/talk-libs => ../talk-libs
