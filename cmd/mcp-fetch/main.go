package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	defaultFetchTimeout  = 20 * time.Second
	defaultFetchMaxBytes = 256 * 1024
	hardFetchMaxBytes    = 2 * 1024 * 1024
	fetchUserAgent       = "LLMux-MCP-Fetch/1.0"
)

type fetchArgs struct {
	URL      string `json:"url"`
	MaxBytes int    `json:"max_bytes,omitempty"`
	Raw      bool   `json:"raw,omitempty"`
}

func main() {
	s := server.NewMCPServer(
		"LLMux Fetch MCP",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	tool := mcp.NewTool(
		"fetch",
		mcp.WithDescription("Fetch a web page or HTTP resource and return readable text content."),
		mcp.WithString(
			"url",
			mcp.Required(),
			mcp.Description("The http or https URL to fetch."),
		),
		mcp.WithNumber(
			"max_bytes",
			mcp.Description("Optional response byte cap. Default 262144, hard max 2097152."),
		),
		mcp.WithBoolean(
			"raw",
			mcp.Description("Return raw response text instead of extracting readable text from HTML."),
		),
	)

	s.AddTool(tool, fetchHandler)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("fetch MCP server error: %v\n", err)
	}
}

func fetchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args fetchArgs
	if err := request.BindArguments(&args); err != nil {
		return mcp.NewToolResultErrorFromErr("invalid fetch arguments", err), nil
	}

	args.URL = strings.TrimSpace(args.URL)
	if args.URL == "" {
		return mcp.NewToolResultError("url is required"), nil
	}

	parsedURL, err := url.Parse(args.URL)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("invalid url", err), nil
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return mcp.NewToolResultError("url must use http or https"), nil
	}
	if parsedURL.Host == "" {
		return mcp.NewToolResultError("url host is required"), nil
	}

	result, err := fetchURL(ctx, parsedURL.String(), args.MaxBytes, args.Raw)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("fetch failed", err), nil
	}
	return result, nil
}

func fetchURL(ctx context.Context, target string, maxBytes int, raw bool) (*mcp.CallToolResult, error) {
	limit := normalizeFetchMaxBytes(maxBytes)
	client := &http.Client{
		Timeout: defaultFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", "text/html,application/json,text/plain;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, truncated, err := readResponseBody(resp.Body, limit)
	if err != nil {
		return nil, err
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	rendered := renderFetchedBody(body, contentType, raw)
	if rendered == "" {
		rendered = "[empty response body]"
	}

	var result strings.Builder
	result.WriteString("URL: ")
	result.WriteString(resp.Request.URL.String())
	result.WriteString("\nStatus: ")
	result.WriteString(resp.Status)
	if contentType != "" {
		result.WriteString("\nContent-Type: ")
		result.WriteString(contentType)
	}
	if truncated {
		result.WriteString("\nTruncated: true")
	}
	result.WriteString("\n\n")
	result.WriteString(rendered)

	text := strings.TrimSpace(result.String())
	if resp.StatusCode >= http.StatusBadRequest {
		return mcp.NewToolResultError(text), nil
	}
	return mcp.NewToolResultText(text), nil
}

func normalizeFetchMaxBytes(maxBytes int) int {
	switch {
	case maxBytes <= 0:
		return defaultFetchMaxBytes
	case maxBytes > hardFetchMaxBytes:
		return hardFetchMaxBytes
	default:
		return maxBytes
	}
}

func readResponseBody(body io.Reader, maxBytes int) ([]byte, bool, error) {
	limited := io.LimitReader(body, int64(maxBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}

func renderFetchedBody(body []byte, contentType string, raw bool) string {
	trimmedType := strings.ToLower(strings.TrimSpace(contentType))
	text := string(body)
	if raw {
		return strings.TrimSpace(text)
	}
	if isHTMLContent(trimmedType, text) {
		return extractHTMLText(text)
	}
	if isTextualContent(trimmedType) {
		return normalizeText(text)
	}
	return fmt.Sprintf("[non-text response omitted, %d bytes]", len(body))
}

func isHTMLContent(contentType, body string) bool {
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml") {
		return true
	}
	prefix := strings.TrimSpace(strings.ToLower(body))
	return strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html")
}

func isTextualContent(contentType string) bool {
	if contentType == "" {
		return true
	}
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "javascript")
}

func extractHTMLText(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return normalizeText(input)
	}

	var lines []string
	var visit func(*html.Node, bool)
	visit = func(node *html.Node, skip bool) {
		if node == nil {
			return
		}

		name := ""
		if node.Type == html.ElementNode {
			name = strings.ToLower(node.Data)
			if shouldSkipElement(name) {
				skip = true
			}
			if isBlockElement(name) {
				lines = append(lines, "")
			}
		}

		if !skip && node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				lines = append(lines, text)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child, skip)
		}

		if node.Type == html.ElementNode && isBlockElement(name) {
			lines = append(lines, "")
		}
	}

	visit(doc, false)

	paragraphs := make([]string, 0, len(lines))
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = current[:0]
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()

	return strings.Join(paragraphs, "\n\n")
}

func shouldSkipElement(name string) bool {
	switch name {
	case "script", "style", "noscript", "template", "svg", "head":
		return true
	default:
		return false
	}
}

func isBlockElement(name string) bool {
	switch name {
	case "p", "div", "section", "article", "header", "footer", "main", "aside", "nav",
		"li", "ul", "ol", "table", "tr", "td", "th", "pre", "blockquote", "br",
		"h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}

func normalizeText(input string) string {
	parts := strings.Split(input, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	return strings.Join(lines, "\n")
}
