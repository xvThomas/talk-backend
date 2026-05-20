package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/swaggest/jsonschema-go"
)

// TypedTool is the generic interface for typed tool implementors used by
// the conversation engine (via Adapt).
type TypedTool[TInput any, TOutput any] interface {
	Name() string
	Description() string
	Call(ctx context.Context, input TInput) (TOutput, error)
}

// Tool is the type-erased interface used internally by the conversation
// engine and LLM converters. It works with raw maps and string output.
type Tool interface {
	Name() string
	Description() string
	InputSchema() (map[string]any, error)  // JSON Schema for the input parameters
	OutputSchema() (map[string]any, error) // JSON Schema for the output
	// Execute runs the tool with the given input and returns a map result.
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
}

// toolAdapter bridges a TypedTool into the type-erased Tool interface.
type toolAdapter[TInput any, TOutput any] struct {
	tool TypedTool[TInput, TOutput]
}

var _ Tool = (*toolAdapter[any, any])(nil)

// Adapt wraps a TypedTool[TInput, TOutput] into a Tool.
func Adapt[TInput any, TOutput any](
	tool TypedTool[TInput, TOutput],
) Tool {
	return &toolAdapter[TInput, TOutput]{tool: tool}
}

func (a *toolAdapter[TInput, TOutput]) Name() string        { return a.tool.Name() }
func (a *toolAdapter[TInput, TOutput]) Description() string { return a.tool.Description() }

func (a *toolAdapter[TInput, TOutput]) InputSchema() (map[string]any, error) {
	var toolInput TInput
	reflector := jsonschema.Reflector{}
	reflector.InlineDefinition(toolInput)

	schema, err := reflector.Reflect(toolInput)
	if err != nil {
		return nil, fmt.Errorf("failed to reflect input schema: %w", err)
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to convert schema to map: %w", err)
	}

	return schemaMap, nil
}

func (a *toolAdapter[TInput, TOutput]) OutputSchema() (map[string]any, error) {
	var toolOutput TOutput
	reflector := jsonschema.Reflector{}

	outputType := reflect.TypeOf(toolOutput)
	if outputType.Kind() == reflect.Slice {
		elemType := outputType.Elem()
		elemInstance := reflect.New(elemType).Elem().Interface()
		reflector.InlineDefinition(elemInstance)
	} else {
		reflector.InlineDefinition(toolOutput)
	}

	schema, err := reflector.Reflect(&toolOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to reflect output schema: %w", err)
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output schema: %w", err)
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to convert output schema to map: %w", err)
	}

	return schemaMap, nil
}

func (a *toolAdapter[TInput, TOutput]) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshalling tool input: %w", err)
	}
	var typed TInput
	if err := json.Unmarshal(raw, &typed); err != nil {
		return nil, fmt.Errorf("unmarshalling tool input into %T: %w", typed, err)
	}

	output, err := a.tool.Call(ctx, typed)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshalling tool output: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("unmarshalling tool output: %w", err)
	}
	return result, nil
}
