package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schemas/loop.schema.json
var schemaFS embed.FS

var (
	loopSchemaOnce sync.Once
	loopSchema     *jsonschema.Schema
	loopSchemaErr  error
)

func validateYAMLFile(filename string) error {
	_, _, err := loadYAMLDocument(filename)
	return err
}

func isYAMLFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}

func validateLoopFile(filename string) error {
	_, root, err := loadYAMLDocument(filename)
	if err != nil {
		return err
	}

	value, err := yamlNodeToJSONValue(root)
	if err != nil {
		return fmt.Errorf("convert yaml for schema validation: %w", err)
	}

	schema, err := compiledLoopSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("invalid loop schema: %w", err)
	}
	return nil
}

func loadYAMLDocument(filename string) ([]byte, *yaml.Node, error) {
	if !isYAMLFilename(filename) {
		return nil, nil, fmt.Errorf("expected a .yaml or .yml file, got %q", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("read yaml file: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil, errors.New("yaml file is empty")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("invalid yaml: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil, errors.New("yaml file does not contain a document")
	}
	if err := rejectDuplicateMappingKeys(doc.Content[0], "$"); err != nil {
		return nil, nil, err
	}
	return data, doc.Content[0], nil
}

func rejectDuplicateMappingKeys(node *yaml.Node, path string) error {
	if node == nil {
		return nil
	}

	if node.Kind == yaml.MappingNode {
		seen := map[string]struct{}{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			keyPath := path + "." + key.Value
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("invalid yaml: duplicate mapping key %q at %s", key.Value, path)
			}
			seen[key.Value] = struct{}{}
			if err := rejectDuplicateMappingKeys(value, keyPath); err != nil {
				return err
			}
		}
		return nil
	}

	for i, child := range node.Content {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		if err := rejectDuplicateMappingKeys(child, childPath); err != nil {
			return err
		}
	}
	return nil
}

func compiledLoopSchema() (*jsonschema.Schema, error) {
	loopSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		schemaData, err := schemaFS.ReadFile("schemas/loop.schema.json")
		if err != nil {
			loopSchemaErr = fmt.Errorf("read embedded loop schema: %w", err)
			return
		}

		var schemaDoc any
		if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
			loopSchemaErr = fmt.Errorf("parse embedded loop schema: %w", err)
			return
		}

		if err := compiler.AddResource("loop.schema.json", schemaDoc); err != nil {
			loopSchemaErr = fmt.Errorf("load embedded loop schema: %w", err)
			return
		}
		loopSchema, loopSchemaErr = compiler.Compile("loop.schema.json")
		if loopSchemaErr != nil {
			loopSchemaErr = fmt.Errorf("compile embedded loop schema: %w", loopSchemaErr)
		}
	})
	return loopSchema, loopSchemaErr
}

func yamlNodeToJSONValue(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return yamlNodeToJSONValue(node.Content[0])
	case yaml.MappingNode:
		value := make(map[string]any, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return nil, fmt.Errorf("mapping key at line %d must be a string", key.Line)
			}
			child, err := yamlNodeToJSONValue(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			value[key.Value] = child
		}
		return value, nil
	case yaml.SequenceNode:
		value := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			childValue, err := yamlNodeToJSONValue(child)
			if err != nil {
				return nil, err
			}
			value = append(value, childValue)
		}
		return value, nil
	case yaml.ScalarNode:
		return yamlScalarToJSONValue(node)
	case yaml.AliasNode:
		return nil, fmt.Errorf("yaml aliases are not supported at line %d", node.Line)
	default:
		return nil, fmt.Errorf("unsupported yaml node kind %d at line %d", node.Kind, node.Line)
	}
}

func yamlScalarToJSONValue(node *yaml.Node) (any, error) {
	switch node.Tag {
	case "!!null":
		return nil, nil
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean at line %d: %w", node.Line, err)
		}
		return value, nil
	case "!!int":
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer at line %d: %w", node.Line, err)
		}
		return value, nil
	case "!!float":
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number at line %d: %w", node.Line, err)
		}
		return value, nil
	default:
		return node.Value, nil
	}
}
