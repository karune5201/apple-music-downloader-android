package i18n

// 本文件是 media 区的文案：ALAC 结构修复与歌词转换。
//
// 约定：文件名、路径、URL、codec 名（alac/aac/flac）、box 名（moov/trak/stbl…
// ）、偏移量、大小、十六进制串一律原样输出，不进这里；这里只有句子本身。
// 数字格式化交给 logx.Bytes()/logx.KV()，占位符以 %s 或 %d 承接。
func init() {
	// ---- 按 ALAC 魔数盒与轨道统计（--verbose 时的容器走查细节）----
	// %s 是 logx.Bytes() 格式化后的 moov 大小。
	register(
		"moov %s，ALAC 轨道 %d 条",
		"moov %s、ALAC トラック %d 件",
		"moov %s, %d ALAC track(s)",
		"media.alacfix.moov_ok",
	)

	// 单条轨道的参数摘要。采样参数名（max_samples_per_frame 等）保持英文原样。
	register(
		"轨道 #%d：%d 个包 max_samples_per_frame=%d sample_size=%d channels=%d",
		"トラック #%d：%d パケット max_samples_per_frame=%d sample_size=%d channels=%d",
		"track #%d: %d packets max_samples_per_frame=%d sample_size=%d channels=%d",
		"media.alacfix.track_params",
	)

	// 该轨道全部包结构完整。
	register(
		"轨道 #%d 完好",
		"トラック #%d は正常",
		"track #%d intact",
		"media.alacfix.track_clean",
	)

	// 该轨道发现了需要处理的包。
	register(
		"轨道 #%d 检测到 %d 个异常包",
		"トラック #%d で %d パケットの異常を検出",
		"track #%d has %d abnormal packets",
		"media.alacfix.track_damaged",
	)

	// 单个包缺 TYPE_END 终止符，准备改写这 3 bit。偏移按十六进制原样给出。
	register(
		"轨道 #%d 包 #%d 偏移 0x%x 大小 %s 正文结束位 %d",
		"トラック #%d パケット #%d オフセット 0x%x サイズ %s 本文終了ビット %d",
		"track #%d packet #%d offset 0x%x size %s body ends at bit %d",
		"media.alacfix.packet_damaged",
	)

	// 单个包扫不到元素边界，无法判断损坏位置。
	register(
		"包 #%d 无法解析，跳过",
		"パケット #%d を解析できず、スキップします",
		"packet #%d cannot be parsed, skipped",
		"media.alacfix.packet_unparsable",
	)

	// 改写后的尾部区间明细（位区间原样给出）。
	register(
		"轨道 #%d 包 #%d 正文结束位 %d 尾部 [%d..%d) 已清零",
		"トラック #%d パケット #%d 本文終了ビット %d 末尾 [%d..%d) をゼロ埋めしました",
		"track #%d packet #%d body ends at bit %d tail [%d..%d) zeroed",
		"media.alacfix.patch_row",
	)

	// 本次运行修好了若干包。
	register(
		"已修复 %d 个包",
		"%d パケットを修復しました",
		"repaired %d packets",
		"media.alacfix.repaired",
	)

	// 本次运行没有需要修的地方。
	register(
		"音频结构完好，无需修复",
		"音声の構造は正常で、修復は不要です",
		"audio integrity intact",
		"media.alacfix.intact",
	)

	// 有包定位到损坏但改写失败（越界等），文件仍然不可用。
	register(
		"%d 个包无法修复",
		"%d パケットは修復できません",
		"%d packets could not be repaired",
		"media.alacfix.unrepairable",
	)

	// ---- 歌词 ----
	// 连不上 lite-server（未配置的情况不报）。
	register(
		"连接 lite-server 失败：%v",
		"lite-server への接続に失敗しました：%v",
		"lite-server connection failed: %v",
		"media.lyrics.lite_failed",
	)
}
