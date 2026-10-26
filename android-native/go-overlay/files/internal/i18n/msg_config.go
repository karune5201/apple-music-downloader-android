package i18n

// 配置文件相关的提示（缺字段、文件不存在）。这类消息在 --json 之外
// 只会出现在启动阶段，但用户最常看到，值得翻译。
func init() {
	register(
		"%s 缺少部分配置项，缺失的先用默认值",
		"%s に一部の設定項目がありません。既定値で続行します",
		"%s is missing some fields; using defaults for them",
		"app.config.missing_fields")

	register(
		"缺失项：%s",
		"不足している項目: %s",
		"missing fields: %s",
		"app.config.missing_fields_list")

	register(
		"找不到 %s，改用默认配置",
		"%s が見つかりません。既定の設定で続行します",
		"%s not found; using defaults",
		"app.config.not_found")
}
