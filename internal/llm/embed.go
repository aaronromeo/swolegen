package llm

import _ "embed"

// Embeds for prompts and schemas used by the llm package.

//go:embed prompts/analyzer-system.txt
var AnalyzerSystem string

//go:embed prompts/analyzer-user.txt
var AnalyzerUser string

//go:embed prompts/generator-system.txt
var GeneratorSystem string

//go:embed prompts/generator-user.txt
var GeneratorUser string

//go:embed prompts/repair-analyzer.txt
var RepairAnalyzer string

//go:embed prompts/repair-generator.txt
var RepairGenerator string

//go:embed schemas/analyzer-v1.json
var AnalyzerSchema string

//go:embed schemas/workout-v1.2.json
var WorkoutSchema string
