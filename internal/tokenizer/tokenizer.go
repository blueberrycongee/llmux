// Package tokenizer provides token counting helpers for LLM requests and responses.
package tokenizer

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"

	"github.com/blueberrycongee/llmux/pkg/types"
)

var (
	encodingCache sync.Map
	defaultOnce   sync.Once
	defaultEnc    *tiktoken.Tiktoken
)

const (
	replyPrimerTokenCount        = 3
	functionDefinitionTokenCount = 9
	toolChoiceNoneTokenCount     = 1
	toolChoiceObjectTokenCount   = 7
	systemMessageToolAdjustment  = 4

	baseImageTokenCount = 85
	defaultImageWidth   = 300
	defaultImageHeight  = 300
	maxShortSideHighRes = 768
	maxLongSideHighRes  = 2000
	maxTileWidth        = 512
	maxTileHeight       = 512
)

type messageCountParams struct {
	tokensPerMessage int
	tokensPerName    int
	countTokens      func(string) int
}

func newMessageCountParams(model string) messageCountParams {
	normalized := normalizeModelName(model)
	params := messageCountParams{
		tokensPerMessage: 3,
		tokensPerName:    1,
		countTokens: func(text string) int {
			return CountTextTokens(model, text)
		},
	}

	if normalized == "gpt-3.5-turbo-0301" {
		params.tokensPerMessage = 4
		params.tokensPerName = -1
	}

	return params
}

// CountTextTokens returns the token count for the given text using tiktoken.
// If no encoding is available, it falls back to a conservative len/4 estimate.
func CountTextTokens(model, text string) int {
	if text == "" {
		return 0
	}
	enc := getEncoding(model)
	if enc == nil {
		return len(text) / 4
	}
	return len(enc.Encode(text, nil, nil))
}

// EstimatePromptTokens estimates prompt tokens for chat requests.
// This matches OpenAI chat token accounting (per-message + tool + reply primer).
func EstimatePromptTokens(model string, req *types.ChatRequest) int {
	if req == nil {
		return 0
	}

	return countChatTokens(model, req.Messages, req.Tools, req.ToolChoice, false)
}

// EstimateEmbeddingTokens estimates token usage for embedding inputs.
// It supports string, []string, []int, and [][]int input formats.
func EstimateEmbeddingTokens(model string, req *types.EmbeddingRequest) int {
	if req == nil || req.Input == nil {
		return 0
	}

	input := req.Input
	if input.Text != nil {
		return CountTextTokens(model, *input.Text)
	}
	if len(input.Texts) > 0 {
		total := 0
		for _, text := range input.Texts {
			total += CountTextTokens(model, text)
		}
		return total
	}
	if len(input.Tokens) > 0 {
		return len(input.Tokens)
	}
	if len(input.TokensList) > 0 {
		total := 0
		for _, tokens := range input.TokensList {
			total += len(tokens)
		}
		return total
	}
	return 0
}

// EstimateCompletionTokens estimates output tokens from a response.
// If no response choices are available, it falls back to the provided text.
func EstimateCompletionTokens(model string, resp *types.ChatResponse, fallbackText string) int {
	if resp != nil && len(resp.Choices) > 0 {
		messages := make([]types.ChatMessage, 0, len(resp.Choices))
		for i := range resp.Choices {
			messages = append(messages, resp.Choices[i].Message)
		}
		total := countChatTokens(model, messages, nil, nil, true)
		if total > 0 {
			return total
		}
	}

	return CountTextTokens(model, fallbackText)
}

// EstimateCompletionTokensFromText estimates assistant output tokens from raw text.
func EstimateCompletionTokensFromText(model, text string) int {
	if text == "" {
		return 0
	}
	raw, err := json.Marshal(text)
	if err != nil {
		return CountTextTokens(model, text)
	}
	msg := types.ChatMessage{
		Role:    "assistant",
		Content: raw,
	}
	return countChatTokens(model, []types.ChatMessage{msg}, nil, nil, true)
}

func countChatTokens(model string, messages []types.ChatMessage, tools []types.Tool, toolChoice json.RawMessage, countResponseTokens bool) int {
	params := newMessageCountParams(model)
	total := 0
	for _, msg := range messages {
		total += countMessageTokens(params, msg)
	}
	if !countResponseTokens {
		total += countExtraTokens(params, messages, tools, toolChoice)
	}
	return total
}

func countMessageTokens(params messageCountParams, msg types.ChatMessage) int {
	total := params.tokensPerMessage
	if msg.Role != "" {
		total += params.countTokens(msg.Role)
	}
	if msg.Name != "" {
		total += params.countTokens(msg.Name)
		total += params.tokensPerName
	}
	total += countContentTokens(params, msg.Content)
	total += countToolCallsTokens(params, msg.ToolCalls)
	if msg.ToolCallID != "" {
		total += params.countTokens(msg.ToolCallID)
	}
	return total
}

func countToolCallsTokens(params messageCountParams, calls []types.ToolCall) int {
	if len(calls) == 0 {
		return 0
	}
	total := 0
	for _, call := range calls {
		if call.Function.Arguments != "" {
			total += params.countTokens(call.Function.Arguments)
		}
	}
	return total
}

func countExtraTokens(params messageCountParams, messages []types.ChatMessage, tools []types.Tool, toolChoice json.RawMessage) int {
	total := replyPrimerTokenCount

	if len(tools) > 0 {
		definition := formatFunctionDefinitions(tools)
		if definition != "" {
			total += params.countTokens(definition)
		}
		total += functionDefinitionTokenCount
		if hasSystemMessage(messages) {
			total -= systemMessageToolAdjustment
		}
	}

	total += countToolChoiceTokens(params, toolChoice)
	return total
}

func countToolChoiceTokens(params messageCountParams, toolChoice json.RawMessage) int {
	if len(toolChoice) == 0 {
		return 0
	}

	var choiceStr string
	if err := json.Unmarshal(toolChoice, &choiceStr); err == nil {
		switch choiceStr {
		case "none":
			return toolChoiceNoneTokenCount
		case "auto", "":
			return 0
		default:
			return 0
		}
	}

	var choiceObj map[string]any
	if err := json.Unmarshal(toolChoice, &choiceObj); err != nil {
		return 0
	}

	functionName := ""
	if fnObj, ok := choiceObj["function"].(map[string]any); ok {
		if name, ok := fnObj["name"].(string); ok {
			functionName = name
		}
	}
	if functionName == "" {
		return 0
	}

	return toolChoiceObjectTokenCount + params.countTokens(functionName)
}

func hasSystemMessage(messages []types.ChatMessage) bool {
	for _, msg := range messages {
		if msg.Role == "system" {
			return true
		}
	}
	return false
}

func countContentTokens(params messageCountParams, raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}

	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return params.countTokens(content)
	}

	var parts []any
	if err := json.Unmarshal(raw, &parts); err == nil {
		return countContentList(params, parts)
	}

	return params.countTokens(string(raw))
}

func countContentList(params messageCountParams, content []any) int {
	total := 0
	for _, item := range content {
		switch v := item.(type) {
		case string:
			total += params.countTokens(v)
		case map[string]any:
			total += countContentItem(params, v)
		default:
			if raw, err := json.Marshal(v); err == nil {
				total += params.countTokens(string(raw))
			}
		}
	}
	return total
}

func countContentItem(params messageCountParams, item map[string]any) int {
	itemType, _ := item["type"].(string)
	switch itemType {
	case "text":
		if text, ok := item["text"].(string); ok {
			return params.countTokens(text)
		}
		if text, ok := item["input_text"].(string); ok {
			return params.countTokens(text)
		}
	case "input_text":
		if text, ok := item["input_text"].(string); ok {
			return params.countTokens(text)
		}
		if text, ok := item["text"].(string); ok {
			return params.countTokens(text)
		}
	case "image_url":
		return countImageTokens(item)
	case "tool_use", "tool_result":
		return countAnthropicContentTokens(params, item)
	}

	if text, ok := item["text"].(string); ok && itemType == "" {
		return params.countTokens(text)
	}
	if raw, err := json.Marshal(item); err == nil {
		return params.countTokens(string(raw))
	}
	return 0
}

func countAnthropicContentTokens(params messageCountParams, content map[string]any) int {
	skipFields := map[string]struct{}{
		"type":          {},
		"id":            {},
		"tool_use_id":   {},
		"cache_control": {},
		"is_error":      {},
	}

	total := 0
	for key, value := range content {
		if _, ok := skipFields[key]; ok {
			continue
		}
		switch v := value.(type) {
		case string:
			total += params.countTokens(v)
		case []any:
			total += countContentList(params, v)
		case map[string]any:
			if raw, err := json.Marshal(v); err == nil {
				total += params.countTokens(string(raw))
			}
		}
	}
	return total
}

func countImageTokens(item map[string]any) int {
	detail := "auto"
	url := ""

	if rawImage, ok := item["image_url"]; ok {
		switch v := rawImage.(type) {
		case string:
			url = v
		case map[string]any:
			if d, ok := v["detail"].(string); ok {
				detail = d
			}
			if u, ok := v["url"].(string); ok {
				url = u
			}
		}
	}

	return calculateImageTokens(url, detail)
}

func calculateImageTokens(data, detail string) int {
	if data == "" {
		return 0
	}

	if detail == "" || detail == "auto" || detail == "low" {
		return baseImageTokenCount
	}
	if detail != "high" {
		return baseImageTokenCount
	}

	width, height := getImageDimensions(data)
	resizedWidth, resizedHeight := resizeImageHighRes(width, height)
	tilesNeeded := calculateTilesNeeded(resizedWidth, resizedHeight, maxTileWidth, maxTileHeight)
	tileTokens := (baseImageTokenCount * 2) * tilesNeeded
	return baseImageTokenCount + tileTokens
}

func getImageDimensions(data string) (int, int) {
	raw, err := loadImageData(data)
	if err != nil || len(raw) == 0 {
		return defaultImageWidth, defaultImageHeight
	}

	if width, height, ok := parseImageDimensions(raw); ok {
		return width, height
	}

	return defaultImageWidth, defaultImageHeight
}

func loadImageData(data string) ([]byte, error) {
	// Explicitly avoid fetching remote URLs during token counting to prevent SSRF/latency.
	if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
		return nil, fmt.Errorf("network fetch disabled for token counting")
	}

	if strings.HasPrefix(data, "data:") {
		if idx := strings.Index(data, ","); idx >= 0 {
			data = data[idx+1:]
		} else {
			return nil, fmt.Errorf("invalid data url")
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func parseImageDimensions(data []byte) (int, int, bool) {
	if len(data) < 10 {
		return 0, 0, false
	}

	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 {
		if len(data) < 24 {
			return 0, 0, false
		}
		width := int(binary.BigEndian.Uint32(data[16:20]))
		height := int(binary.BigEndian.Uint32(data[20:24]))
		return width, height, true
	}

	if len(data) >= 6 && string(data[:4]) == "GIF8" && data[5] == 'a' {
		width := int(binary.LittleEndian.Uint16(data[6:8]))
		height := int(binary.LittleEndian.Uint16(data[8:10]))
		return width, height, true
	}

	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return parseJPEGDimensions(data)
	}

	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return parseWebPDimensions(data)
	}

	return 0, 0, false
}

func parseJPEGDimensions(data []byte) (int, int, bool) {
	if len(data) < 4 {
		return 0, 0, false
	}

	i := 2
	for i+1 < len(data) {
		if data[i] != 0xff {
			i++
			continue
		}
		for i < len(data) && data[i] == 0xff {
			i++
		}
		if i >= len(data) {
			break
		}
		marker := data[i]
		i++
		if marker == 0xd9 || marker == 0xda {
			break
		}
		if i+1 >= len(data) {
			break
		}
		segmentLen := int(binary.BigEndian.Uint16(data[i : i+2]))
		if segmentLen < 2 || i+segmentLen > len(data) {
			break
		}

		if marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc {
			if i+7 > len(data) {
				break
			}
			height := int(binary.BigEndian.Uint16(data[i+3 : i+5]))
			width := int(binary.BigEndian.Uint16(data[i+5 : i+7]))
			return width, height, true
		}

		i += segmentLen
	}
	return 0, 0, false
}

func parseWebPDimensions(data []byte) (int, int, bool) {
	if len(data) < 30 {
		return 0, 0, false
	}
	switch string(data[12:16]) {
	case "VP8X":
		if len(data) < 30 {
			return 0, 0, false
		}
		width := int(data[24]) | int(data[25])<<8 | int(data[26])<<16
		height := int(data[27]) | int(data[28])<<8 | int(data[29])<<16
		return width + 1, height + 1, true
	case "VP8 ":
		if len(data) < 30 {
			return 0, 0, false
		}
		width := int(binary.LittleEndian.Uint16(data[26:28]) & 0x3fff)
		height := int(binary.LittleEndian.Uint16(data[28:30]) & 0x3fff)
		return width, height, true
	case "VP8L":
		if len(data) < 25 {
			return 0, 0, false
		}
		bits := binary.LittleEndian.Uint32(data[21:25])
		width := int(bits&0x3fff) + 1
		height := int((bits>>14)&0x3fff) + 1
		return width, height, true
	default:
		return 0, 0, false
	}
}

func resizeImageHighRes(width, height int) (int, int) {
	if width <= maxShortSideHighRes && height <= maxShortSideHighRes {
		return width, height
	}
	if width == 0 || height == 0 {
		return width, height
	}

	aspectRatio := float64(width) / float64(height)
	if width <= height {
		resizedWidth := maxShortSideHighRes
		resizedHeight := int(float64(resizedWidth) / aspectRatio)
		if resizedHeight > maxLongSideHighRes {
			resizedHeight = maxLongSideHighRes
			resizedWidth = int(float64(resizedHeight) * aspectRatio)
		}
		return resizedWidth, resizedHeight
	}

	resizedHeight := maxShortSideHighRes
	resizedWidth := int(float64(resizedHeight) * aspectRatio)
	if resizedWidth > maxLongSideHighRes {
		resizedWidth = maxLongSideHighRes
		resizedHeight = int(float64(resizedWidth) / aspectRatio)
	}
	return resizedWidth, resizedHeight
}

func calculateTilesNeeded(resizedWidth, resizedHeight, tileWidth, tileHeight int) int {
	if tileWidth <= 0 || tileHeight <= 0 {
		return 0
	}
	tilesAcross := (resizedWidth + tileWidth - 1) / tileWidth
	tilesDown := (resizedHeight + tileHeight - 1) / tileHeight
	if tilesAcross == 0 || tilesDown == 0 {
		return 0
	}
	return tilesAcross * tilesDown
}

func formatFunctionDefinitions(tools []types.Tool) string {
	if len(tools) == 0 {
		return ""
	}
	lines := []string{"namespace functions {", ""}
	for _, tool := range tools {
		function := tool.Function
		if function.Description != "" {
			lines = append(lines, fmt.Sprintf("// %s", function.Description))
		}
		parameters := map[string]any{}
		if len(function.Parameters) > 0 {
			_ = json.Unmarshal(function.Parameters, &parameters)
		}
		properties, _ := parameters["properties"].(map[string]any)
		if len(properties) > 0 {
			lines = append(
				lines,
				fmt.Sprintf("type %s = (_: {", function.Name),
				formatObjectParameters(parameters, 0),
				"}) => any;",
			)
		} else {
			lines = append(lines, fmt.Sprintf("type %s = () => any;", function.Name))
		}
		lines = append(lines, "")
	}
	lines = append(lines, "} // namespace functions")
	return strings.Join(lines, "\n")
}

func formatObjectParameters(parameters map[string]any, indent int) string {
	properties, _ := parameters["properties"].(map[string]any)
	if len(properties) == 0 {
		return ""
	}

	requiredSet := map[string]bool{}
	if required, ok := parameters["required"].([]any); ok {
		for _, item := range required {
			if name, ok := item.(string); ok {
				requiredSet[name] = true
			}
		}
	}

	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(properties))
	for _, key := range keys {
		props, _ := properties[key].(map[string]any)
		if description, ok := props["description"].(string); ok && description != "" {
			lines = append(lines, strings.Repeat(" ", indent)+"// "+description)
		}
		question := "?"
		if requiredSet[key] {
			question = ""
		}
		lines = append(lines, strings.Repeat(" ", indent)+fmt.Sprintf("%s%s: %s,", key, question, formatType(props, indent)))
	}

	return strings.Join(lines, "\n")
}

func formatType(props map[string]any, indent int) string {
	typ, _ := props["type"].(string)
	switch typ {
	case "string":
		if enumVals, ok := props["enum"].([]any); ok && len(enumVals) > 0 {
			return formatEnum(enumVals)
		}
		return "string"
	case "array":
		items, _ := props["items"].(map[string]any)
		return fmt.Sprintf("%s[]", formatType(items, indent))
	case "object":
		return fmt.Sprintf("{\n%s\n}", formatObjectParameters(props, indent+2))
	case "integer", "number":
		if enumVals, ok := props["enum"].([]any); ok && len(enumVals) > 0 {
			return formatEnum(enumVals)
		}
		return "number"
	case "boolean":
		return "boolean"
	case "null":
		return "null"
	default:
		return "any"
	}
}

func formatEnum(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%q", v))
		default:
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(parts, " | ")
}

func getEncoding(model string) *tiktoken.Tiktoken {
	base := normalizeModelName(model)
	if cached, ok := encodingCache.Load(base); ok {
		if enc, ok := cached.(*tiktoken.Tiktoken); ok {
			return enc
		}
		return getDefaultEncoding()
	}

	if strings.Contains(base, "gpt-4o") {
		if enc, err := tiktoken.GetEncoding("o200k_base"); err == nil {
			encodingCache.Store(base, enc)
			return enc
		}
	}

	enc, err := tiktoken.EncodingForModel(base)
	if err != nil {
		enc = getDefaultEncoding()
	}
	if enc != nil {
		encodingCache.Store(base, enc)
	}
	return enc
}

func getDefaultEncoding() *tiktoken.Tiktoken {
	defaultOnce.Do(func() {
		enc, err := tiktoken.GetEncoding("cl100k_base")
		if err == nil {
			defaultEnc = enc
		}
	})
	return defaultEnc
}

func normalizeModelName(model string) string {
	if model == "" {
		return model
	}
	if idx := strings.LastIndex(model, "/"); idx >= 0 && idx+1 < len(model) {
		return model[idx+1:]
	}
	return model
}
