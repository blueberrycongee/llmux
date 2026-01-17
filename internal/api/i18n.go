package api //nolint:revive // package name is intentional

import (
	"net/http"
	"strings"
)

func detectLocaleFromRequest(r *http.Request) string {
	if r == nil {
		return "i18n"
	}
	if v := strings.TrimSpace(r.Header.Get("X-LLMux-Locale")); v != "" {
		v = strings.ToLower(v)
		if v == "cn" || strings.HasPrefix(v, "zh") {
			return "cn"
		}
		return "i18n"
	}
	al := strings.ToLower(r.Header.Get("Accept-Language"))
	if strings.Contains(al, "zh") {
		return "cn"
	}
	return "i18n"
}

func localizeManagementMessage(locale string, message string) string {
	if locale != "cn" {
		return message
	}

	switch message {
	case "invalid request body":
		return "请求体无效"
	case "request body too large":
		return "请求体过大"
	case "authentication required":
		return "需要认证"
	case "management permission required":
		return "需要管理权限"

	case "id parameter is required":
		return "缺少参数：id"

	case "key is required":
		return "缺少参数：key"
	case "keys is required":
		return "缺少参数：keys"
	case "key parameter is required":
		return "缺少参数：key"
	case "key not found":
		return "未找到该 Key"
	case "failed to generate api key":
		return "生成 API Key 失败"
	case "failed to create api key":
		return "创建 API Key 失败"
	case "failed to update api key":
		return "更新 API Key 失败"
	case "failed to get key info":
		return "获取 Key 信息失败"
	case "failed to list keys":
		return "获取 Key 列表失败"
	case "failed to block key":
		return "封禁 Key 失败"
	case "failed to unblock key":
		return "解封 Key 失败"
	case "failed to regenerate key":
		return "重置 Key 失败"

	case "team_id is required":
		return "缺少参数：team_id"
	case "team_ids is required":
		return "缺少参数：team_ids"
	case "team_id parameter is required":
		return "缺少参数：team_id"
	case "team not found":
		return "未找到该团队"
	case "failed to create team":
		return "创建团队失败"
	case "failed to update team":
		return "更新团队失败"
	case "failed to get team info":
		return "获取团队信息失败"
	case "failed to list teams":
		return "获取团队列表失败"
	case "failed to block team":
		return "封禁团队失败"
	case "failed to unblock team":
		return "解封团队失败"

	case "user_id is required":
		return "缺少参数：user_id"
	case "user_ids is required":
		return "缺少参数：user_ids"
	case "user_id parameter is required":
		return "缺少参数：user_id"
	case "user not found":
		return "未找到该用户"
	case "failed to create user":
		return "创建用户失败"
	case "failed to update user":
		return "更新用户失败"
	case "failed to get user info":
		return "获取用户信息失败"
	case "failed to list users":
		return "获取用户列表失败"

	case "organization_alias is required":
		return "缺少参数：organization_alias"
	case "organization_id is required":
		return "缺少参数：organization_id"
	case "organization_ids is required":
		return "缺少参数：organization_ids"
	case "organization_id parameter is required":
		return "缺少参数：organization_id"
	case "organization not found":
		return "未找到该组织"
	case "failed to create organization":
		return "创建组织失败"
	case "failed to create organization budget":
		return "创建组织预算失败"
	case "failed to update organization":
		return "更新组织失败"
	case "failed to get organization info":
		return "获取组织信息失败"
	case "failed to list organizations":
		return "获取组织列表失败"
	case "members is required":
		return "缺少参数：members"
	case "organization_id and user_id are required":
		return "缺少参数：organization_id 和 user_id"
	case "organization_id and user_ids are required":
		return "缺少参数：organization_id 和 user_ids"
	case "membership not found":
		return "未找到成员关系"
	case "failed to update member":
		return "更新成员失败"
	case "failed to list members":
		return "获取成员列表失败"

	case "failed to list audit logs":
		return "获取审计日志失败"
	case "failed to get audit log":
		return "获取审计日志详情失败"
	case "audit log not found":
		return "未找到审计日志"
	case "failed to get audit stats":
		return "获取审计统计失败"
	case "older_than_days must be positive":
		return "older_than_days 必须为正数"
	case "failed to delete audit logs":
		return "删除审计日志失败"

	case "invalid start_date format, use YYYY-MM-DD":
		return "start_date 格式无效，请使用 YYYY-MM-DD"
	case "invalid end_date format, use YYYY-MM-DD":
		return "end_date 格式无效，请使用 YYYY-MM-DD"
	case "invalid start_date format":
		return "start_date 格式无效"
	case "invalid end_date format":
		return "end_date 格式无效"
	case "failed to get spend logs":
		return "获取消费日志失败"
	case "failed to get spend by keys":
		return "获取 Key 消费统计失败"
	case "failed to get spend by teams":
		return "获取团队消费统计失败"
	case "failed to get spend by users":
		return "获取用户消费统计失败"
	case "failed to get global activity":
		return "获取全局活动失败"
	case "failed to get spend by model":
		return "获取模型消费统计失败"
	case "failed to get spend by provider":
		return "获取提供方消费统计失败"

	case "client not available":
		return "客户端不可用"
	case "deployment_id is required":
		return "缺少参数：deployment_id"
	case "deployment not found":
		return "未找到部署"
	case "failed to update cooldown":
		return "更新冷却时间失败"
	case "config manager not available":
		return "配置管理器不可用"
	case "config checksum mismatch":
		return "配置校验和不匹配"
	case "failed to reload config":
		return "重新加载配置失败"

	case "team_id or organization_id is required":
		return "team_id 或 organization_id 必须提供一个"
	case "token is required":
		return "缺少参数：token"
	case "id is required":
		return "缺少参数：id"
	case "ids is required":
		return "缺少参数：ids"
	case "failed to create invitation link":
		return "创建邀请链接失败"
	case "failed to accept invitation":
		return "接受邀请失败"
	case "failed to get invitation link":
		return "获取邀请链接失败"
	case "invitation not found":
		return "未找到邀请"
	case "failed to list invitation links":
		return "获取邀请列表失败"
	case "failed to deactivate invitation":
		return "停用邀请失败"
	case "invitation link has been deactivated":
		return "邀请链接已停用"
	}

	return message
}
