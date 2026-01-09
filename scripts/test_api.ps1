# LLMux 功能测试脚本 (PowerShell)
# 复制这些命令到 PowerShell 中执行

Write-Host "=== 1. 健康检查 ===" -ForegroundColor Green
Invoke-RestMethod -Uri "http://localhost:8080/health/live"

Write-Host "`n=== 2. 创建用户 Alice ===" -ForegroundColor Green
$user1 = Invoke-RestMethod -Uri "http://localhost:8080/user/new" -Method Post -ContentType "application/json" -Body '{"user_email":"alice@example.com","user_alias":"Alice Wang","user_role":"internal_user","max_budget":100}'
$user1 | ConvertTo-Json
$aliceId = $user1.user_id

Write-Host "`n=== 3. 创建用户 Bob ===" -ForegroundColor Green
$user2 = Invoke-RestMethod -Uri "http://localhost:8080/user/new" -Method Post -ContentType "application/json" -Body '{"user_email":"bob@example.com","user_alias":"Bob Chen","user_role":"internal_user","max_budget":50}'
$user2 | ConvertTo-Json
$bobId = $user2.user_id

Write-Host "`n=== 4. 列出所有用户 ===" -ForegroundColor Green
Invoke-RestMethod -Uri "http://localhost:8080/user/list" | ConvertTo-Json -Depth 5

Write-Host "`n=== 5. 创建团队 ===" -ForegroundColor Green
$team = Invoke-RestMethod -Uri "http://localhost:8080/team/new" -Method Post -ContentType "application/json" -Body '{"team_alias":"Frontend Team","max_budget":500}'
$team | ConvertTo-Json
$teamId = $team.team_id

Write-Host "`n=== 6. 添加 Alice 到团队 ===" -ForegroundColor Green
$addMember = @{
    team_id = $teamId
    member = @(@{user_id = $aliceId; role = "member"})
} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/team/member_add" -Method Post -ContentType "application/json" -Body $addMember

Write-Host "`n=== 7. 列出团队 ===" -ForegroundColor Green
Invoke-RestMethod -Uri "http://localhost:8080/team/list" | ConvertTo-Json -Depth 5

Write-Host "`n=== 8. 创建 API Key ===" -ForegroundColor Green
$key = Invoke-RestMethod -Uri "http://localhost:8080/key/generate" -Method Post -ContentType "application/json" -Body '{"key_name":"test-key-1","max_budget":20}'
$key | ConvertTo-Json
Write-Host "保存这个 Key: $($key.key)" -ForegroundColor Yellow

Write-Host "`n=== 9. 列出 API Keys ===" -ForegroundColor Green
Invoke-RestMethod -Uri "http://localhost:8080/key/list" | ConvertTo-Json -Depth 5

Write-Host "`n=== 10. 创建组织 ===" -ForegroundColor Green
$org = Invoke-RestMethod -Uri "http://localhost:8080/organization/new" -Method Post -ContentType "application/json" -Body '{"organization_alias":"Acme Corp","max_budget":2000}'
$org | ConvertTo-Json

Write-Host "`n=== 11. 审计日志 ===" -ForegroundColor Green
Invoke-RestMethod -Uri "http://localhost:8080/audit/logs?limit=10" | ConvertTo-Json -Depth 5

Write-Host "`n=== 测试完成! 现在刷新 Dashboard 查看数据 ===" -ForegroundColor Cyan
Write-Host "打开: http://localhost:3000" -ForegroundColor Cyan
