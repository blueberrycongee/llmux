package api

import (
	"net/http"
	"time"
)

type ToolPresetClientTemplate struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Envs           []string          `json:"envs,omitempty"`
	ToolsToExecute []string          `json:"tools_to_execute,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
}

type ToolPreset struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Category    string                   `json:"category"`
	Tags        []string                 `json:"tags"`
	Client      ToolPresetClientTemplate `json:"client"`
	Tools       []ToolMarketplaceItem    `json:"tools"`
}

type ImportToolPresetRequest struct {
	PresetID string `json:"preset_id"`
}

func defaultToolPresets() []ToolPreset {
	now := time.Now().Format(time.RFC3339)
	return []ToolPreset{
		{
			ID:          "filesystem-core",
			Name:        "Filesystem Tools",
			Description: "常见文件读写、目录浏览、文本编辑类工具，适合 coding / research agent。",
			Category:    "coding",
			Tags:        []string{"filesystem", "editor", "coding", "mcp"},
			Client:      ToolPresetClientTemplate{ID: "filesystem", Name: "Filesystem MCP", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "C:\\Users\\1\\Desktop\\go_code\\llmux"}, ToolsToExecute: []string{"*"}},
			Tools: []ToolMarketplaceItem{
				{ID: "preset-filesystem-read", Name: "Filesystem Read", Description: "读取文件与目录内容", Category: "coding", SourceClientID: "filesystem", SourceToolName: "read_file", Tags: []string{"filesystem", "read"}, Enabled: true, CreatedAt: now, UpdatedAt: now},
				{ID: "preset-filesystem-write", Name: "Filesystem Write", Description: "写入或覆盖文本文件", Category: "coding", SourceClientID: "filesystem", SourceToolName: "write_file", Tags: []string{"filesystem", "write"}, Enabled: true, CreatedAt: now, UpdatedAt: now},
				{ID: "preset-filesystem-list", Name: "Filesystem List", Description: "列出目录与文件信息", Category: "coding", SourceClientID: "filesystem", SourceToolName: "list_directory", Tags: []string{"filesystem", "list"}, Enabled: true, CreatedAt: now, UpdatedAt: now},
			},
		},
		{
			ID:          "fetch-core",
			Name:        "Fetch / HTTP Tools",
			Description: "让 research agent 真正具备网页抓取与 HTTP 获取能力。",
			Category:    "research",
			Tags:        []string{"fetch", "http", "research", "web"},
			Client:      ToolPresetClientTemplate{ID: "fetch", Name: "Fetch MCP", Type: "stdio", Command: "go", Args: []string{"run", "./cmd/mcp-fetch"}, ToolsToExecute: []string{"*"}},
			Tools:       []ToolMarketplaceItem{{ID: "preset-fetch-web", Name: "Fetch Web", Description: "抓取网页与 HTTP 资源", Category: "research", SourceClientID: "fetch", SourceToolName: "fetch", Tags: []string{"http", "web", "research"}, Enabled: true, CreatedAt: now, UpdatedAt: now}},
		},
		{
			ID:          "playwright-browser",
			Name:        "Playwright Browser Tools",
			Description: "让 agent 具备浏览器操作、页面截图、表单点击与测试能力。",
			Category:    "research",
			Tags:        []string{"browser", "playwright", "automation", "testing"},
			Client:      ToolPresetClientTemplate{ID: "playwright", Name: "Playwright MCP", Type: "stdio", Command: "npx", Args: []string{"-y", "@playwright/mcp@latest"}, ToolsToExecute: []string{"*"}},
			Tools:       []ToolMarketplaceItem{{ID: "preset-browser-open", Name: "Browser Automation", Description: "浏览器打开、点击、输入与页面分析", Category: "research", SourceClientID: "playwright", SourceToolName: "browser_navigate", Tags: []string{"browser", "automation"}, Enabled: true, CreatedAt: now, UpdatedAt: now}},
		},
		{
			ID:          "sqlite-data",
			Name:        "SQLite Data Tools",
			Description: "为 coding / analysis agent 提供本地 SQLite 查询能力。",
			Category:    "coding",
			Tags:        []string{"sqlite", "database", "analysis"},
			Client:      ToolPresetClientTemplate{ID: "sqlite", Name: "SQLite MCP", Type: "stdio", Command: "uvx", Args: []string{"mcp-server-sqlite", "--db-path", "./tmp/agent.db"}, ToolsToExecute: []string{"*"}},
			Tools:       []ToolMarketplaceItem{{ID: "preset-sqlite-query", Name: "SQLite Query", Description: "执行 SQLite 查询与表结构检查", Category: "coding", SourceClientID: "sqlite", SourceToolName: "query", Tags: []string{"sqlite", "sql"}, Enabled: true, CreatedAt: now, UpdatedAt: now}},
		},
		{
			ID:          "github-dev",
			Name:        "GitHub Dev Tools",
			Description: "让 coding / review agent 真正具备 GitHub 读写、issue 与 PR 处理能力。",
			Category:    "coding",
			Tags:        []string{"github", "repo", "code-review"},
			Client:      ToolPresetClientTemplate{ID: "github", Name: "GitHub MCP", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}, Envs: []string{"GITHUB_TOKEN=${GITHUB_TOKEN}"}, ToolsToExecute: []string{"*"}},
			Tools:       []ToolMarketplaceItem{{ID: "preset-github-repo", Name: "GitHub Repo Ops", Description: "仓库、issue、PR 与文件操作", Category: "coding", SourceClientID: "github", SourceToolName: "get_file_contents", Tags: []string{"github", "repo"}, Enabled: true, CreatedAt: now, UpdatedAt: now}},
		},
	}
}

func defaultMarketplaceItems() []ToolMarketplaceItem {
	presets := defaultToolPresets()
	items := make([]ToolMarketplaceItem, 0)
	for _, preset := range presets {
		items = append(items, preset.Tools...)
	}
	return items
}

func (h *ManagementHandler) ListToolPresets(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"data": defaultToolPresets()})
}

func (h *ManagementHandler) ImportToolPreset(w http.ResponseWriter, r *http.Request) {
	var req ImportToolPresetRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	var preset *ToolPreset
	for i := range defaultToolPresets() {
		item := defaultToolPresets()[i]
		if item.ID == req.PresetID {
			preset = &item
			break
		}
	}
	if preset == nil {
		h.writeError(w, r, http.StatusNotFound, "tool preset not found")
		return
	}
	imported := make([]ToolMarketplaceItem, 0, len(preset.Tools))
	for _, item := range preset.Tools {
		result, err := upsertMarketplaceItem(UpsertToolMarketplaceRequest{ID: item.ID, Name: item.Name, Description: item.Description, Category: item.Category, SourceClientID: item.SourceClientID, SourceToolName: item.SourceToolName, Tags: item.Tags, Enabled: &item.Enabled}, false)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		imported = append(imported, result)
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"preset": preset, "imported": imported})
}
