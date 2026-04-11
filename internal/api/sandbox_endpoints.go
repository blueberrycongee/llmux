package api //nolint:revive

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/goccy/go-json"
)

type sandboxDistrict struct {
	Name  string  `json:"name"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Color string  `json:"color"`
}

type sandboxAgent struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Action   string  `json:"action"`
	Status   string  `json:"status"`
	Memory   string  `json:"memory"`
	Progress float64 `json:"progress"`
	Speed    float64 `json:"speed"`
}

type sandboxState struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Premise      string            `json:"premise"`
	Theme        string            `json:"theme"`
	Protocol     string            `json:"protocol"`
	Institutions []string          `json:"institutions"`
	Rules        []string          `json:"rules"`
	Templates    []string          `json:"templates"`
	Archive      []string          `json:"archive"`
	Districts    []sandboxDistrict `json:"districts"`
	Agents       []sandboxAgent    `json:"agents"`
	Tick         int               `json:"tick"`
	Logs         []sandboxLog      `json:"logs"`
}

type sandboxLog struct {
	Tick int    `json:"tick"`
	Text string `json:"text"`
}

type generateSandboxRequest struct {
	Prompt string `json:"prompt"`
}

type stepSandboxRequest struct {
	ID string `json:"id"`
}

var sandboxStore = struct {
	sync.Mutex
	items map[string]*sandboxState
}{items: map[string]*sandboxState{}}

func (h *ManagementHandler) GenerateSandbox(w http.ResponseWriter, r *http.Request) {
	var req generateSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	state := buildSandboxState(req.Prompt)
	sandboxStore.Lock()
	sandboxStore.items[state.ID] = state
	sandboxStore.Unlock()
	h.writeJSON(w, http.StatusOK, state)
}

func (h *ManagementHandler) GetSandboxState(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeError(w, r, http.StatusBadRequest, "id is required")
		return
	}
	sandboxStore.Lock()
	state := sandboxStore.items[id]
	sandboxStore.Unlock()
	if state == nil {
		h.writeError(w, r, http.StatusNotFound, "sandbox not found")
		return
	}
	h.writeJSON(w, http.StatusOK, state)
}

func (h *ManagementHandler) StepSandbox(w http.ResponseWriter, r *http.Request) {
	var req stepSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		h.writeError(w, r, http.StatusBadRequest, "id is required")
		return
	}
	sandboxStore.Lock()
	state := sandboxStore.items[req.ID]
	if state == nil {
		sandboxStore.Unlock()
		h.writeError(w, r, http.StatusNotFound, "sandbox not found")
		return
	}
	state.Tick++
	actor := state.Agents[state.Tick%len(state.Agents)]
	state.Logs = append([]sandboxLog{{Tick: state.Tick, Text: fmt.Sprintf("%s：%s", actor.Name, actor.Action)}}, state.Logs...)
	state.Archive = append([]string{fmt.Sprintf("T%d：%s 的行动被沉淀为经验。", state.Tick, actor.Name)}, state.Archive...)
	for i := range state.Agents {
		state.Agents[i].Progress += state.Agents[i].Speed * 12
		for state.Agents[i].Progress >= 1 {
			state.Agents[i].Progress -= 1
		}
	}
	updated := *state
	sandboxStore.Unlock()
	h.writeJSON(w, http.StatusOK, &updated)
}

func buildSandboxState(prompt string) *sandboxState {
	if containsIndustry(prompt) {
		return &sandboxState{ID: "sandbox-industry", Name: "工业转型镇", Premise: "失业率上升、资源紧张，社会情绪逐渐不稳定。", Theme: "industry", Protocol: "安置协商 + 物资配给 + 风险隔离", Institutions: []string{"工会", "市政厅", "警务站", "救济中心"}, Rules: []string{"公共补贴", "集会许可", "应急征用"}, Templates: []string{"罢工升级", "补贴争议", "仓储短缺"}, Archive: []string{"先处理救济与舆情，再处理冲突。", "失业会迅速触发群体行动。"}, Districts: []sandboxDistrict{{Name: "工厂区", X: 10, Y: 12, W: 24, H: 18, Color: "#8b5a3c"}, {Name: "旧城", X: 40, Y: 10, W: 26, H: 16, Color: "#726b58"}, {Name: "市政区", X: 72, Y: 12, W: 18, H: 16, Color: "#5c6474"}, {Name: "仓储区", X: 18, Y: 58, W: 24, H: 16, Color: "#6e6455"}, {Name: "救济区", X: 60, Y: 58, W: 24, H: 18, Color: "#7f735a"}}, Agents: []sandboxAgent{{ID: "a1", Name: "Union He", Role: "工会代表", From: "工厂区", To: "市政区", Action: "正在前往市政区提交补贴与安置诉求。", Status: "发起协商", Memory: "失业会快速引发群体行动。", Progress: 0.18, Speed: 0.012}, {ID: "a2", Name: "Supply Mo", Role: "仓储协调者", From: "仓储区", To: "救济区", Action: "正在把关键物资转运至救济区。", Status: "资源调度", Memory: "短缺时要先守住关键资源。", Progress: 0.62, Speed: 0.011}, {ID: "a3", Name: "Marshal Qiao", Role: "秩序维护者", From: "旧城", To: "工厂区", Action: "正在赶往工厂区防止抗议升级。", Status: "应急响应", Memory: "冲突要分区隔离。", Progress: 0.42, Speed: 0.009}}, Tick: 0, Logs: []sandboxLog{{Tick: 0, Text: "已生成社会沙盘：工业转型镇"}}}
	}
	return &sandboxState{ID: "sandbox-harbor", Name: "港口贸易城", Premise: "依赖贸易、信息传播和外来人口流动的海港社会。", Theme: "harbor", Protocol: "公开协调 + 市场调度 + 社区安抚", Institutions: []string{"市政厅", "港务局", "市场公会", "新闻站"}, Rules: []string{"信息公开", "治安巡查", "物资配给"}, Templates: []string{"补给延迟", "谣言扩散", "港区拥堵"}, Archive: []string{"港口危机时优先稳定信息渠道。", "价格波动会先在市场区放大。"}, Districts: []sandboxDistrict{{Name: "港区", X: 10, Y: 12, W: 24, H: 18, Color: "#6d8aa6"}, {Name: "市场区", X: 40, Y: 10, W: 26, H: 16, Color: "#8f6d49"}, {Name: "市政区", X: 72, Y: 12, W: 18, H: 16, Color: "#7d6b58"}, {Name: "居住区", X: 18, Y: 58, W: 24, H: 16, Color: "#7f735a"}, {Name: "档案区", X: 60, Y: 58, W: 24, H: 18, Color: "#8f6d49"}}, Agents: []sandboxAgent{{ID: "a1", Name: "Mayor Lin", Role: "市政协调者", From: "市政区", To: "市场区", Action: "正在前往市场区协调补给与价格波动。", Status: "巡场沟通", Memory: "危机时先稳住信息渠道。", Progress: 0.1, Speed: 0.012}, {ID: "a2", Name: "Broker Shen", Role: "市场协调者", From: "市场区", To: "港区", Action: "正在去港区确认到港物资与交易节奏。", Status: "核对供给", Memory: "价格波动会放大焦虑。", Progress: 0.55, Speed: 0.01}, {ID: "a3", Name: "Marshal Qiao", Role: "秩序维护者", From: "港区", To: "居住区", Action: "正在沿主路巡查，防止恐慌扩散。", Status: "移动巡逻", Memory: "高压只适合短期止损。", Progress: 0.32, Speed: 0.008}}, Tick: 0, Logs: []sandboxLog{{Tick: 0, Text: "已生成社会沙盘：港口贸易城"}}}
}

func containsIndustry(prompt string) bool {
	for _, kw := range []string{"工业", "失业", "工厂", "罢工"} {
		if prompt != "" && contains(prompt, kw) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || len(s) > len(sub) && (contains(s[1:], sub) || s[:len(sub)] == sub))
}
