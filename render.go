package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type loopDefinition struct {
	Metadata   loopMetadata    `yaml:"metadata"`
	Spec       loopSpec        `yaml:"spec"`
	Phases     []loopPhase     `yaml:"phases"`
	Escalation *loopEscalation `yaml:"escalation"`
}

type loopMetadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type loopSpec struct {
	Objective string               `yaml:"objective"`
	Inputs    map[string]loopInput `yaml:"inputs"`
}

type loopInput struct {
	Type        any    `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

type loopPhase struct {
	Name        string           `yaml:"name"`
	Title       string           `yaml:"title"`
	Objective   string           `yaml:"objective"`
	Actions     []string         `yaml:"actions"`
	Completion  []string         `yaml:"completion"`
	Outputs     []map[string]any `yaml:"outputs"`
	Transitions []loopTransition `yaml:"transitions"`
}

type loopTransition struct {
	To        string `yaml:"to"`
	Condition string `yaml:"condition"`
}

type loopEscalation struct {
	Principle        string                `yaml:"principle"`
	EscalationInputs []loopEscalationInput `yaml:"escalation_inputs"`
}

type loopEscalationInput struct {
	Name        string `yaml:"name"`
	Type        any    `yaml:"type"`
	Description string `yaml:"description"`
}

type renderOptions struct {
	noColor bool
	details bool
}

type ansiPalette struct {
	phase      string
	transition string
	escalation string
	muted      string
	reset      string
}

func renderSource(ctx context.Context, opts options, stdout io.Writer) error {
	filename, err := renderSourceFilename(ctx, opts)
	if err != nil {
		return err
	}
	if err := validateLoopFile(filename); err != nil {
		return err
	}
	loop, err := readLoopDefinition(filename)
	if err != nil {
		return err
	}
	return renderLoop(stdout, loop, renderOptions{
		noColor: opts.renderNoColor || os.Getenv("NO_COLOR") != "",
		details: opts.renderDetails,
	})
}

func renderSourceFilename(ctx context.Context, opts options) (string, error) {
	if opts.filename != "" {
		return opts.filename, nil
	}
	if opts.targetRef == "" {
		return "", errors.New("render source is empty")
	}
	if _, err := pullPackage(ctx, opts); err != nil {
		return "", err
	}
	return cachedFilePath(opts.targetRef)
}

func readLoopDefinition(filename string) (loopDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return loopDefinition{}, fmt.Errorf("read loop file: %w", err)
	}
	var loop loopDefinition
	if err := yaml.Unmarshal(data, &loop); err != nil {
		return loopDefinition{}, fmt.Errorf("parse loop file: %w", err)
	}
	return loop, nil
}

func renderLoop(stdout io.Writer, loop loopDefinition, opts renderOptions) error {
	indexByName, err := phaseIndexByName(loop.Phases)
	if err != nil {
		return err
	}
	palette := renderPalette(opts.noColor)

	fmt.Fprintf(stdout, "%sLoop:%s %s (%s)\n", palette.phase, palette.reset, loop.Metadata.Title, loop.Metadata.Version)
	if strings.TrimSpace(loop.Spec.Objective) != "" {
		fmt.Fprintf(stdout, "%sObjective:%s %s\n", palette.muted, palette.reset, oneLine(loop.Spec.Objective))
	}
	fmt.Fprintf(stdout, "%sInputs:%s %s\n\n", palette.muted, palette.reset, renderInputs(loop.Spec.Inputs))

	for i, phase := range loop.Phases {
		fmt.Fprintf(stdout, "%s[%d] %s%s\n", palette.phase, i+1, phase.Title, palette.reset)
		if phase.Name != "" {
			fmt.Fprintf(stdout, "     %sname:%s %s\n", palette.muted, palette.reset, phase.Name)
		}
		if opts.details {
			renderPhaseDetails(stdout, phase, palette)
		}

		edges, err := phaseEdges(i, phase, loop.Phases, indexByName)
		if err != nil {
			return err
		}
		for edgeIndex, edge := range edges {
			prefix := "     ├─"
			if edgeIndex == len(edges)-1 && loop.Escalation == nil {
				prefix = "     └─"
			}
			fmt.Fprintf(stdout, "%s%s %s%s\n", palette.transition, prefix, edge, palette.reset)
		}
		if loop.Escalation != nil {
			fmt.Fprintf(stdout, "%s     ↯ escalation:%s %s\n", palette.escalation, palette.reset, renderEscalationInputs(loop.Escalation.EscalationInputs))
		}
		fmt.Fprintln(stdout)
	}

	if loop.Escalation != nil {
		fmt.Fprintf(stdout, "%sEscalation:%s %s\n", palette.escalation, palette.reset, oneLine(loop.Escalation.Principle))
	}
	return nil
}

func phaseIndexByName(phases []loopPhase) (map[string]int, error) {
	indexByName := make(map[string]int, len(phases))
	for i, phase := range phases {
		if phase.Name == "" {
			continue
		}
		if _, exists := indexByName[phase.Name]; exists {
			return nil, fmt.Errorf("duplicate phase name %q", phase.Name)
		}
		indexByName[phase.Name] = i
	}
	return indexByName, nil
}

func phaseEdges(index int, phase loopPhase, phases []loopPhase, indexByName map[string]int) ([]string, error) {
	if len(phase.Transitions) > 0 {
		edges := make([]string, 0, len(phase.Transitions))
		for _, transition := range phase.Transitions {
			targetIndex, ok := indexByName[transition.To]
			if !ok {
				return nil, fmt.Errorf("phase %q transition targets unknown phase %q", phase.Name, transition.To)
			}
			edges = append(edges, fmt.Sprintf("%s -> [%d] %s", renderCondition(transition.Condition), targetIndex+1, phases[targetIndex].Title))
		}
		return edges, nil
	}
	if index+1 < len(phases) {
		return []string{fmt.Sprintf("next -> [%d] %s", index+2, phases[index+1].Title)}, nil
	}
	return []string{"end"}, nil
}

func renderCondition(condition string) string {
	condition = oneLine(condition)
	if condition == "" {
		return "if condition"
	}
	if strings.HasPrefix(strings.ToLower(condition), "if ") {
		return condition
	}
	return "if " + condition
}

func renderInputs(inputs map[string]loopInput) string {
	if len(inputs) == 0 {
		return "none"
	}
	names := sortedKeys(inputs)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		input := inputs[name]
		required := ""
		if input.Required {
			required = "*"
		}
		parts = append(parts, fmt.Sprintf("%s%s %s", name, required, renderType(input.Type)))
	}
	return strings.Join(parts, ", ")
}

func renderEscalationInputs(inputs []loopEscalationInput) string {
	if len(inputs) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(inputs))
	for _, input := range inputs {
		parts = append(parts, input.Name)
	}
	return strings.Join(parts, ", ")
}

func renderPhaseDetails(stdout io.Writer, phase loopPhase, palette ansiPalette) {
	if strings.TrimSpace(phase.Objective) != "" {
		fmt.Fprintf(stdout, "     %sobjective:%s %s\n", palette.muted, palette.reset, oneLine(phase.Objective))
	}
	if len(phase.Actions) > 0 {
		fmt.Fprintf(stdout, "     %sactions:%s %s\n", palette.muted, palette.reset, renderListSummary(phase.Actions))
	}
	if len(phase.Completion) > 0 {
		fmt.Fprintf(stdout, "     %scompletion:%s %s\n", palette.muted, palette.reset, renderListSummary(phase.Completion))
	}
	if len(phase.Outputs) > 0 {
		fmt.Fprintf(stdout, "     %soutputs:%s %s\n", palette.muted, palette.reset, renderOutputs(phase.Outputs))
	}
}

func renderOutputs(outputs []map[string]any) string {
	parts := make([]string, 0, len(outputs))
	for _, output := range outputs {
		for name, value := range output {
			parts = append(parts, fmt.Sprintf("%s %s", name, renderType(value)))
		}
	}
	return strings.Join(parts, ", ")
}

func renderListSummary(items []string) string {
	const maxItems = 2
	parts := make([]string, 0, len(items))
	for i, item := range items {
		if i >= maxItems {
			parts = append(parts, fmt.Sprintf("+%d more", len(items)-maxItems))
			break
		}
		parts = append(parts, oneLine(item))
	}
	return strings.Join(parts, "; ")
}

func renderType(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return "[" + strings.Join(typed, "|") + "]"
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return "[" + strings.Join(parts, "|") + "]"
	default:
		return fmt.Sprint(value)
	}
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func renderPalette(noColor bool) ansiPalette {
	if noColor {
		return ansiPalette{}
	}
	return ansiPalette{
		phase:      "\x1b[36m",
		transition: "\x1b[33m",
		escalation: "\x1b[35m",
		muted:      "\x1b[2m",
		reset:      "\x1b[0m",
	}
}
