# Issue #15：安全签发、查询和撤销

产品基线 dev@4dc5a3f；本 Issue 是 v1.0.0 首项，无业务依赖。按用户约束使用项目内 `.worktrees/15/`，不复用项目外旧 #1 worktree。用 GoFrame CLI 2.9.7 的内置 gf init 在本工作树 `.cache/scaffold/service` 生成工程种子，再按项目 DDD/单 server 要求取用并固定运行时 GF 2.10.3；没有引入模板的数据库、hello API 或部署占位。

## 实施参数

- query：limit 默认20、范围1–100；offset默认0、非负；按 created_at 倒序，同秒 token 字典序倒序；返回 items/total/limit/offset，item 不含源 URL。
- metadata：可选 JSON object，编码不超过4096字节；请求body最大64KiB，filename最长255字节；未知字段拒绝。code=0成功，其余统一用 gcode detail 映射真实HTTP状态。
- 单主Valkey（RESP兼容客户端由GoFrame adapter提供）；要求noeviction，运行身份/安全epoch异常时拒绝授权。冷安全状态建立或存储状态丢失覆盖完整601秒重放窗口再开放，新epoch不读取旧token；时钟由内部注入以在CI验证，无测试HTTP端点。

## TDD 恢复点

已先写 integration build-tag 的真实 TLS HTTP/Valkey 断言，覆盖 S01-AC01–05。初始Module仅返回501，准备推送同一PR取得目标行为缺失的红态；没有在本地运行测试。后续实现必须使这些断言在当前head CI通过，不能删除断言过绿。

S01没有文件下载/转换/预览成功能力；有效token的/v/保持503，失效404，不能宣称v1或H5验收已完成。后续S02接入受控资源，不创建临时裸链兼容。
