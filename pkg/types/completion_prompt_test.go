package types_test

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestCompletionPrompt_UnmarshalString(t *testing.T) {
	var prompt types.CompletionPrompt

	err := json.Unmarshal([]byte(`"hello"`), &prompt)
	require.NoError(t, err)

	require.NotNil(t, prompt.Text)
	assert.Equal(t, "hello", *prompt.Text)
	assert.Nil(t, prompt.Texts)

	text, err := prompt.AsText()
	require.NoError(t, err)
	assert.Equal(t, "hello", text)
}

func TestCompletionPrompt_UnmarshalStringArray(t *testing.T) {
	var prompt types.CompletionPrompt

	err := json.Unmarshal([]byte(`["hello","world"]`), &prompt)
	require.NoError(t, err)

	assert.Nil(t, prompt.Text)
	require.Len(t, prompt.Texts, 2)
	assert.Equal(t, []string{"hello", "world"}, prompt.Texts)

	text, err := prompt.AsText()
	require.NoError(t, err)
	assert.Equal(t, "hello\nworld", text)
}

func TestCompletionPrompt_UnmarshalInvalid(t *testing.T) {
	var prompt types.CompletionPrompt

	err := json.Unmarshal([]byte(`123`), &prompt)
	require.Error(t, err)
}

func TestCompletionPrompt_ValidateEmptyArray(t *testing.T) {
	var prompt types.CompletionPrompt

	err := json.Unmarshal([]byte(`[]`), &prompt)
	require.NoError(t, err)

	require.Error(t, prompt.Validate())
}
