package i18n

// 转码、MV、动态封面与媒体探测的文案。
//
// 键空间按调用文件划分：app.media（internal/app/media.go 的 m3u8 探测）、
// app.mv（mv.go）、app.artwork（artwork.go）、app.convert（convert.go）。
// 只登记“给人看的句子”；文件名、URL、codec 名、质量表格一律不进目录。
func init() {
	// ---------------------------------------------------------- internal/app/media.go

	register(
		"连接 lite-server 失败（%v）",
		"lite-server への接続に失敗しました（%v）",
		"failed to connect to lite-server: %v",
		"app.media.lite_server_error",
	)
	register(
		"lite-server 返回了空响应",
		"lite-server から空のレスポンスが返りました",
		"lite-server returned an empty response",
		"app.media.lite_empty_response",
	)
	register(
		"发现 Dolby Atmos 流：%s（%d Kbps）",
		"Dolby Atmos ストリームを検出：%s（%d Kbps）",
		"found Dolby Atmos variant: %s (%d Kbps)",
		"app.media.debug_atmos_variant",
	)
	register(
		"发现 Dolby Audio 流：%s（%d Kbps）",
		"Dolby Audio ストリームを検出：%s（%d Kbps）",
		"found Dolby Audio variant: %s (%d Kbps)",
		"app.media.debug_dolby_audio_variant",
	)
	register(
		"发现 AAC 流：%s（%d bps）",
		"AAC ストリームを検出：%s（%d bps）",
		"found AAC variant: %s (%d bps)",
		"app.media.debug_aac_variant",
	)

	// ------------------------------------------------------------- internal/app/mv.go

	register(
		"获取 MV 清单失败（%v）",
		"MV マニフェストの取得に失敗しました（%v）",
		"failed to get MV manifest: %v",
		"app.mv.manifest_failed",
	)
	register(
		"MV 已存在于本地",
		"MV は既にローカルにあります",
		"MV already exists locally",
		"app.mv.already_exists",
	)
	register(
		"MV 缩略图保存失败（%v）",
		"MV サムネイルの保存に失敗しました（%v）",
		"failed to save MV thumbnail: %v",
		"app.mv.thumbnail_failed",
	)
	register(
		"正在复用 MV...",
		"MV を多重化しています...",
		"MV remuxing...",
		"app.mv.remuxing",
	)
	register(
		"MV 复用失败（%v）",
		"MV の多重化に失敗しました（%v）",
		"MV mux failed: %v",
		"app.mv.mux_failed",
	)
	register(
		"MV 复用完成。",
		"MV の多重化が完了しました。",
		"MV remuxed.",
		"app.mv.remuxed",
	)
	register(
		"MV 标签写入失败（%v）",
		"MV タグの書き込みに失敗しました（%v）",
		"MV tag writing failed: %v",
		"app.mv.tag_failed",
	)
	register(
		"视频 DRM：PlayReady",
		"動画 DRM：PlayReady",
		"video DRM: PlayReady",
		"app.mv.drm_playready",
	)

	// -------------------------------------------------------- internal/app/artwork.go

	register(
		"未取到方形动态封面（%v）",
		"正方形モーションアートワークを取得できませんでした（%v）",
		"no motion video square: %v",
		"app.artwork.motion_square_missing",
	)
	register(
		"方形动态封面转 GIF 失败（%v）",
		"正方形モーションアートワークの GIF 変換に失敗しました（%v）",
		"animated artwork square to gif failed: %v",
		"app.artwork.gif_failed",
	)
	register(
		"未取到竖版动态封面（%v）",
		"縦型モーションアートワークを取得できませんでした（%v）",
		"no motion video tall: %v",
		"app.artwork.motion_tall_missing",
	)
	register(
		"动态封面 %s 已存在于本地",
		"モーションアートワーク %s は既にローカルにあります",
		"animated artwork %s already exists locally",
		"app.artwork.already_exists",
	)
	register(
		"正在下载动态封面 %s",
		"モーションアートワーク %s をダウンロードしています",
		"downloading animated artwork %s",
		"app.artwork.downloading",
	)
	register(
		"动态封面 %s 下载完成",
		"モーションアートワーク %s のダウンロードが完了しました",
		"animated artwork %s downloaded",
		"app.artwork.downloaded",
	)
	register(
		"M3U8 播放列表写入失败（%v）",
		"M3U8 プレイリストの書き込みに失敗しました（%v）",
		"failed to write M3U8 playlist: %v",
		"app.artwork.m3u8_write_failed",
	)
	register(
		"M3U8 播放列表已保存",
		"M3U8 プレイリストを保存しました",
		"M3U8 playlist saved",
		"app.artwork.m3u8_saved",
	)

	// -------------------------------------------------------- internal/app/convert.go

	register(
		"目标格式为 copy，不产生新格式，跳过转码",
		"形式が copy のため新しい形式にならず、変換をスキップします",
		"convert (copy) requested; skipping because it produces no new format",
		"app.convert.copy_skipped",
	)
	register(
		"跳过转码：已是 %s",
		"変換をスキップ：既に %s です",
		"conversion skipped (already %s)",
		"app.convert.skip_already",
	)
	register(
		"跳过转码：源为有损且目标为无损，配置要求跳过",
		"変換をスキップ：ソースは非可逆で変換先は可逆、スキップする設定です",
		"skipping conversion: source appears lossy and target is lossless; configured to skip",
		"app.convert.skip_lossy_to_lossless",
	)
	register(
		"有损源转无损容器不会提升音质",
		"非可逆ソースを可逆コンテナに変換しても音質は向上しません",
		"converting lossy source to lossless container will not improve quality",
		"app.convert.warn_lossy_to_lossless",
	)
	register(
		"未找到 ffmpeg：%s，跳过转码",
		"ffmpeg が見つかりません：%s、変換をスキップします",
		"ffmpeg not found at %s; skipping conversion",
		"app.convert.ffmpeg_missing",
	)
	register(
		"转码配置错误（%v）",
		"変換設定のエラー（%v）",
		"conversion config error: %v",
		"app.convert.config_error",
	)
	register(
		"正在转码为 %s",
		"%s に変換しています",
		"converting -> %s",
		"app.convert.converting",
	)
	register(
		"转码失败（%v）",
		"変換に失敗しました（%v）",
		"conversion failed: %v",
		"app.convert.failed",
	)
	register(
		"检测到 ALAC 错误",
		"ALAC エラーを検出しました",
		"detected ALAC error",
		"app.convert.bad_alac_detected",
	)
	register(
		"删除转码产物失败（%v）",
		"変換結果の削除に失敗しました（%v）",
		"failed to remove converted file: %v",
		"app.convert.remove_output_failed",
	)
	register(
		"因 ALAC 错误删除了转码产物",
		"ALAC エラーのため変換結果を削除しました",
		"converted file removed due to bad ALAC",
		"app.convert.removed_bad_alac",
	)
	register(
		"转码日志：%s",
		"変換ログ：%s",
		"convert logs: %s",
		"app.convert.logs",
	)
	register(
		"转码日志已保存到 %s",
		"変換ログを %s に保存しました",
		"convert logs are stored in %s",
		"app.convert.logs_saved",
	)
	register(
		"转码完成，用时 %s：%s",
		"変換が完了しました（%s）：%s",
		"conversion completed in %s: %s",
		"app.convert.completed",
	)
	register(
		"转码后删除原文件失败（%v）",
		"変換後の元ファイル削除に失敗しました（%v）",
		"failed to remove original after conversion: %v",
		"app.convert.remove_original_failed",
	)
	register(
		"原文件已删除",
		"元ファイルを削除しました",
		"original removed",
		"app.convert.original_removed",
	)
}
