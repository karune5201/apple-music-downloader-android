package i18n

// lite-server 客户端（internal/wrapper）的文案。
//
// 键空间固定为 net.lite.…，只放给人看的句子；端点路径、完整 URL、HTTP 状态码、
// code=-1 这类协议片段一律原样作为参数传进来，不进目录。
func init() {
	// 重试时补一行完整 URL，方便对着 lite-server 日志排查。
	register(
		"lite-server 请求 %s",
		"lite-server リクエスト %s",
		"lite-server request %s",
		"net.lite.url",
	)
	// 一次可重试的失败：端点、第几次、共几次、错误。
	register(
		"lite-server %s 第 %d/%d 次尝试失败：%v，准备重试",
		"lite-server %s の試行 %d/%d が失敗しました：%v、再試行します",
		"lite-server %s attempt %d/%d failed: %v, retrying",
		"net.lite.retry",
	)
	// 重试次数用尽，最终失败。
	register(
		"lite-server %s 连续 %d 次尝试均失败：%v",
		"lite-server %s は %d 回試行しましたが失敗しました：%v",
		"lite-server %s failed after %d attempts: %v",
		"net.lite.giveup",
	)
}
