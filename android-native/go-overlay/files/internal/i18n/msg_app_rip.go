package i18n

// 文案目录：app.rip.*（曲目流水线 internal/app/rip.go）。
//
// 这里只放“句子”。文件名、路径、URL、codec 名（ALAC/AAC/Atmos/MP4/ffmpeg）、
// 命令行开关名一律作为 %s/%v 参数传入，不翻译；Go 的 error 值也只走 %v。
// 括号里的 %s 常来自 logx.Bytes / logx.Since / logx.KV 之类的格式化助手。
func init() {
	// ---------------------------------------------------------------- 曲目
	register(
		"第 %d/%d 首（%s）",
		"%d/%d 曲目（%s）",
		"track %d of %d (%s)",
		"app.rip.track_banner",
	)
	register(
		"写入 %s（%s，%s）",
		"%s を書き込み（%s、%s）",
		"wrote %s (%s, %s)",
		"app.rip.track_written",
	)
	register(
		"文件 %s",
		"ファイル %s",
		"file %s",
		"app.rip.song_file",
	)
	register(
		"下载参数 %s %s",
		"ダウンロード設定 %s %s",
		"download params %s %s",
		"app.rip.track_params",
	)
	register(
		"目录 %s",
		"ディレクトリ %s",
		"directory %s",
		"app.rip.track_dir",
	)
	register(
		"清单 %s",
		"マニフェスト %s",
		"manifest %s",
		"app.rip.track_manifest",
	)
	register(
		"不可用",
		"利用できません",
		"unavailable",
		"app.rip.unavailable",
	)
	register(
		"不可用，改用 aac-lc",
		"利用できないため aac-lc で再試行します",
		"unavailable, retrying with aac-lc",
		"app.rip.unavailable_try_aac",
	)
	register(
		"文件存在性检查失败：%v",
		"ファイルの存在確認に失敗：%v",
		"file existence check failed: %v",
		"app.rip.exists_check_failed",
	)
	register(
		"曲目已存在，跳过",
		"曲は既に存在するためスキップします",
		"track already exists, skipping",
		"app.rip.exists",
	)
	register(
		"转码文件已存在，跳过",
		"変換済みファイルが存在するためスキップします",
		"converted track already exists, skipping",
		"app.rip.exists_converted",
	)
	register(
		"从清单提取音质失败：%v",
		"マニフェストから音質を取得できません：%v",
		"failed to extract quality from manifest: %v",
		"app.rip.quality_extract_failed",
	)
	register(
		"从清单提取信息失败：%v",
		"マニフェストからの情報取得に失敗：%v",
		"failed to extract info from manifest: %v",
		"app.rip.manifest_extract_failed",
	)
	register(
		"清单获取失败：%v",
		"マニフェスト取得に失敗：%v",
		"failed to get manifest: %v",
		"app.rip.manifest_get_failed",
	)
	register(
		"专辑信息获取失败：%v",
		"アルバム情報の取得に失敗：%v",
		"failed to get album response: %v",
		"app.rip.album_response_failed",
	)
	register(
		"播放列表曲目的专辑信息获取失败：%v",
		"プレイリスト曲のアルバム情報取得に失敗：%v",
		"failed to get album data for playlist track: %v",
		"app.rip.album_data_failed",
	)
	register(
		"歌曲信息获取失败：%v",
		"曲情報の取得に失敗：%v",
		"failed to get song response: %v",
		"app.rip.song_response_failed",
	)
	register(
		"歌曲下载失败：%v",
		"曲のダウンロードに失敗：%v",
		"failed to rip song: %v",
		"app.rip.song_rip_failed",
	)

	// ---------------------------------------------------------------- 歌词
	register(
		"无可用歌词",
		"歌詞が見つかりません",
		"no lyrics available",
		"app.rip.lyrics_unavailable",
	)
	register(
		"歌词获取失败：%v",
		"歌詞の取得に失敗：%v",
		"lyrics fetch failed: %v",
		"app.rip.lyrics_failed",
	)
	register(
		"歌词写入失败：%v",
		"歌詞の書き込みに失敗：%v",
		"lyrics write failed: %v",
		"app.rip.lyrics_write_failed",
	)

	// ------------------------------------------------------------ 下载与后处理
	register(
		"aac-lc 下载需要 lite-server，当前未配置",
		"aac-lc のダウンロードには lite-server が必要ですが未設定です",
		"aac-lc download requires lite-server, which is not configured",
		"app.rip.aac_lc_need_lite_server",
	)
	register(
		"经 lite-server 下载 aac-lc 失败：%v",
		"lite-server 経由の aac-lc ダウンロードに失敗：%v",
		"aac-lc download via lite-server failed: %v",
		"app.rip.aac_lc_download_failed",
	)
	register(
		"v4 解密失败：%v",
		"v4 復号に失敗：%v",
		"v4 decryption failed: %v",
		"app.rip.fairplay_failed",
	)
	register(
		"fMP4 解碎片失败：%v",
		"fMP4 のデフラグに失敗：%v",
		"defragment failed: %v",
		"app.rip.defrag_failed",
	)
	register(
		"ALAC 修复失败：%v",
		"ALAC 修復に失敗：%v",
		"ALAC fix failed: %v",
		"app.rip.alac_fix_failed",
	)
	register(
		"媒体标签写入失败：%v",
		"メディアタグの書き込みに失敗：%v",
		"failed to write tags into media: %v",
		"app.rip.tags_write_failed",
	)
	register(
		"封面写入失败：%v",
		"カバーの書き込みに失敗：%v",
		"cover write failed: %v",
		"app.rip.cover_write_failed",
	)
	register(
		"歌手封面写入失败：%v",
		"アーティストカバーの書き込みに失敗：%v",
		"failed to write artist cover: %v",
		"app.rip.artist_cover_failed",
	)
	register(
		"封面文件删除失败：%s",
		"カバーファイルの削除に失敗：%s",
		"failed to delete cover file: %s",
		"app.rip.cover_delete_failed",
	)
	register(
		"未能从 lite-server 取到更佳音质的 m3u8，改用 Web API 的 m3u8",
		"lite-server から高音質 m3u8 を取得できないため Web API の m3u8 を使用します",
		"failed to get best quality m3u8 from lite-server, using the Web API m3u8",
		"app.rip.m3u8_fallback",
	)

	// ------------------------------------------------------------ 电台
	register(
		"电台类型 %s",
		"ステーション種別 %s",
		"station type %s",
		"app.rip.station_type",
	)
	register(
		"电台文件已存在，跳过",
		"ステーションは既に存在するためスキップします",
		"radio station already exists, skipping",
		"app.rip.station_exists",
	)
	register(
		"写入 %s（%s）",
		"%s を書き込み（%s）",
		"wrote %s (%s)",
		"app.rip.station_written",
	)
	register(
		"电台资源地址获取失败：%v",
		"ステーションのリソース URL 取得に失敗：%v",
		"failed to get station assets url: %v",
		"app.rip.station_assets_failed",
	)
	register(
		"电台变体播放列表解析失败：%v",
		"ステーションのバリアントプレイリスト解決に失敗：%v",
		"failed to resolve station variant playlist: %v",
		"app.rip.station_playlist_failed",
	)
	register(
		"电台流解密密钥获取失败：%v",
		"ステーション復号鍵の取得に失敗：%v",
		"failed to get station stream decryption key: %v",
		"app.rip.station_key_failed",
	)
	register(
		"电台流下载失败：%v",
		"ステーションストリームのダウンロードに失敗：%v",
		"failed to download station stream: %v",
		"app.rip.station_download_failed",
	)
	register(
		"电台封面读取失败：%v",
		"ステーションカバーの読み込みに失敗：%v",
		"failed to read station cover: %v",
		"app.rip.station_cover_read_failed",
	)
	register(
		"打开电台流以写入标签失败：%v",
		"タグ書き込み用にステーションストリームを開けません：%v",
		"failed to open station stream for tagging: %v",
		"app.rip.station_tag_open_failed",
	)
	register(
		"电台标签写入失败：%v",
		"ステーションタグの書き込みに失敗：%v",
		"failed to embed station tags: %v",
		"app.rip.station_tag_write_failed",
	)

	// ------------------------------------------------------------ MV
	register(
		"未配置 lite-server，跳过 MV",
		"lite-server 未設定のため MV をスキップします",
		"lite-server not set, skipping MV",
		"app.rip.mv_no_lite_server",
	)
	register(
		"MV 下载失败：%v",
		"MV のダウンロードに失敗：%v",
		"MV download failed: %v",
		"app.rip.mv_download_failed",
	)

	// ------------------------------------------------- 专辑 / 播放列表遍历
	register(
		"播放列表信息获取失败：%v",
		"プレイリスト情報の取得に失敗：%v",
		"failed to get playlist response: %v",
		"app.rip.playlist_response_failed",
	)
	register(
		"歌手目录 %s",
		"アーティストフォルダ %s",
		"artist folder %s",
		"app.rip.artist_folder",
	)
	register(
		"专辑目录 %s",
		"アルバムフォルダ %s",
		"album folder %s",
		"app.rip.album_folder",
	)
	register(
		"播放列表目录 %s",
		"プレイリストフォルダ %s",
		"playlist folder %s",
		"app.rip.playlist_folder",
	)
	register(
		"编码 %s",
		"コーデック %s",
		"codec %s",
		"app.rip.codec",
	)
	register(
		"第 %d/%d 首",
		"%d/%d 曲目",
		"track %d of %d",
		"app.rip.debug_track",
	)
	register(
		"第 %d 首清单获取失败：%v",
		"%d 曲目のマニフェスト取得に失敗：%v",
		"failed to get manifest for track %d: %v",
		"app.rip.debug_manifest_failed",
	)
	register(
		"第 %d 首音质信息提取失败：%v",
		"%d 曲目の音質情報取得に失敗：%v",
		"failed to extract quality info for track %d: %v",
		"app.rip.debug_quality_failed",
	)
}
