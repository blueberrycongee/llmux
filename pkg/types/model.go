package types

import (
	"fmt"
	"strings"
)

const MaxModelNameLength = 256

// ValidateModelName checks that a model name is within acceptable bounds.
func ValidateModelName(model string) error {
	if len(model) > MaxModelNameLength {
		return fmt.Errorf("model is too long (max %d characters)", MaxModelNameLength)
	}
	return nil
}

// SplitProviderModel splits LiteLLM-style "provider/model" strings.
// Returns ("", model) when no provider prefix is present.
func SplitProviderModel(model string) (provider string, modelName string) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", ""
	}
	idx := strings.Index(model, "/")
	if idx <= 0 || idx >= len(model)-1 {
		return "", model
	}
	return model[:idx], model[idx+1:]
}
