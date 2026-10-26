package i18n

// Widevine 下载与解密的文案（net.drm.*）。
//
// 这里的每条都是流水线的一步状态：单曲下载、整段流下载、解密结果。
// URL、KID、PSSH、license 一律不进文案，由调用方拼在参数里。
func init() {
	// 单曲：文件已下载 / 解密失败 / 已解密。
	register("已下载", "ダウンロードしました", "downloaded",
		"net.drm.song_downloaded")
	register("解密失败", "復号に失敗しました", "decryption failed",
		"net.drm.decrypt_failed")
	register("已解密", "復号しました", "decrypted",
		"net.drm.decrypted")

	// 整段流：分段下载完成 / 未加密直接写入 / 解密完成。
	register("分段下载完成", "セグメントのダウンロードが完了しました", "stream downloaded",
		"net.drm.stream_downloaded")
	register("流未加密，直接写入", "暗号化されていないため、そのまま書き込みます", "stream is not encrypted, writing as is",
		"net.drm.stream_clear")
	register("流解密完成", "ストリームの復号が完了しました", "stream decrypted",
		"net.drm.stream_decrypted")
}
