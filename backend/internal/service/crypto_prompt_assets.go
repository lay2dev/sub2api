package service

import (
	_ "embed"
	"strings"
)

//go:embed prompts/crypto_analysis_system_prompt.txt
var cryptoAnalysisSystemPromptAsset string

var cryptoAnalysisSystemPrompt = strings.TrimSpace(cryptoAnalysisSystemPromptAsset)
