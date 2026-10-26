package i18n

// 主流程（internal/app 的 app.go / run.go / url.go / search.go）用到的文案。
//
// 键一律是 app.flow.*；URL、路径、商店代码、codec 名（ALAC/AAC/Atmos）、
// 命令行 flag 名不翻译，Go 的 error 值一律作为 %v 参数传进来。
func init() {
	// ---------------------------------------------------------------- 用法与启动
	register(
		"用法：%s [选项] [url1 url2 ...]",
		"使い方: %s [オプション] [url1 url2 ...]",
		"usage: %s [options] [url1 url2 ...]",
		"app.flow.usage",
	)
	register(
		"搜索用法：%s --search [album|song|artist] 关键词",
		"検索の使い方: %s --search [album|song|artist] クエリ",
		"search usage: %s --search [album|song|artist] query",
		"app.flow.usage_search",
	)
	register(
		"选项：",
		"オプション:",
		"options:",
		"app.flow.usage_options",
	)

	// ---------------------------------------------------------------- 早期失败
	register(
		"配置迁移失败：%v",
		"設定の移行に失敗しました: %v",
		"config migration failed: %v",
		"app.flow.migrate_config_failed",
	)
	register(
		"更新检查失败：%v",
		"更新確認に失敗しました: %v",
		"update check failed: %v",
		"app.flow.update_check_failed",
	)
	register(
		"自更新失败：%v",
		"自己更新に失敗しました: %v",
		"self-update failed: %v",
		"app.flow.self_update_failed",
	)
	register(
		"配置加载失败：%v",
		"設定の読み込みに失敗しました: %v",
		"failed to load config: %v",
		"app.flow.load_config_failed",
	)
	register(
		"查询 lite-server 状态失败：%v",
		"lite-server の状態取得に失敗しました: %v",
		"failed to query lite-server status: %v",
		"app.flow.lite_status_failed",
	)
	register(
		"lite-server 区域：%s",
		"lite-server のリージョン: %s",
		"lite-server regions: %s",
		"app.flow.lite_regions",
	)
	register(
		"代理配置失败：%v",
		"プロキシ設定に失敗しました: %v",
		"proxy config failed: %v",
		"app.flow.proxy_config_failed",
	)
	register(
		"获取 token 失败：%v",
		"トークンの取得に失敗しました: %v",
		"failed to get token: %v",
		"app.flow.token_failed",
	)
	register(
		"未提供 URL，至少需要一个",
		"URL が指定されていません。1 つ以上必要です",
		"no URL provided, at least one is required",
		"app.flow.no_urls",
	)
	register(
		"退出",
		"終了します",
		"exiting",
		"app.flow.exiting",
	)

	// ---------------------------------------------------------------- 歌手页解析
	register(
		"获取歌手名失败：%v",
		"アーティスト名の取得に失敗しました: %v",
		"failed to get artist name: %v",
		"app.flow.artist_name_failed",
	)
	register(
		"获取歌手专辑失败：%v",
		"アーティストのアルバム取得に失敗しました: %v",
		"failed to get artist albums: %v",
		"app.flow.artist_albums_failed",
	)
	register(
		"获取歌手 MV 失败：%v",
		"アーティストの MV 取得に失敗しました: %v",
		"failed to get artist music videos: %v",
		"app.flow.artist_mvs_failed",
	)

	// ---------------------------------------------------------------- 队列与条目
	register(
		"队列 %d/%d",
		"キュー %d/%d",
		"queue %d/%d",
		"app.flow.queue",
	)
	register(
		"专辑",
		"アルバム",
		"album",
		"app.flow.item_album",
	)
	register(
		"播放列表",
		"プレイリスト",
		"playlist",
		"app.flow.item_playlist",
	)
	register(
		"单曲",
		"楽曲",
		"song",
		"app.flow.item_song",
	)
	register(
		"电台",
		"ステーション",
		"station",
		"app.flow.item_station",
	)
	register(
		"音乐视频",
		"ミュージックビデオ",
		"music video",
		"app.flow.item_mv",
	)
	register(
		"无法识别的链接类型",
		"判別できないリンク種別です",
		"unrecognized link type",
		"app.flow.item_type_unknown",
	)

	// ---------------------------------------------------------------- 单条目失败
	register(
		"单曲链接格式无效",
		"楽曲 URL の形式が不正です",
		"invalid song URL format",
		"app.flow.song_url_invalid",
	)
	register(
		"链接无效：%v",
		"URL が不正です: %v",
		"invalid URL: %v",
		"app.flow.url_invalid",
	)
	register(
		"单曲下载失败：%v",
		"楽曲の取得に失敗しました: %v",
		"song download failed: %v",
		"app.flow.rip_song_failed",
	)
	register(
		"专辑下载失败：%v",
		"アルバムの取得に失敗しました: %v",
		"album download failed: %v",
		"app.flow.rip_album_failed",
	)
	register(
		"播放列表下载失败：%v",
		"プレイリストの取得に失敗しました: %v",
		"playlist download failed: %v",
		"app.flow.rip_playlist_failed",
	)
	register(
		"电台下载失败：%v",
		"ステーションの取得に失敗しました: %v",
		"station download failed: %v",
		"app.flow.rip_station_failed",
	)
	register(
		"MV 下载失败：%v",
		"MV のダウンロードに失敗しました: %v",
		"MV download failed: %v",
		"app.flow.mv_failed",
	)
	register(
		"未配置 lite-server，跳过 MV 下载",
		"lite-server が未設定のため MV をスキップします",
		"lite-server not set, skipping MV download",
		"app.flow.lite_server_unset_mv",
	)
	register(
		"未配置 media-user-token，跳过电台下载",
		"media-user-token が未設定のためステーションをスキップします",
		"media-user-token not set, skipping station download",
		"app.flow.media_user_token_unset_station",
	)
	register(
		"获取 manifest 失败：%v",
		"マニフェストの取得に失敗しました: %v",
		"failed to get manifest: %v",
		"app.flow.manifest_failed",
	)

	// ---------------------------------------------------------------- 收尾
	register(
		"完成 %d/%d  警告 %d  错误 %d",
		"完了 %d/%d  警告 %d  エラー %d",
		"done %d/%d  warnings %d  errors %d",
		"app.flow.summary",
	)
	register(
		"检测到错误，退出",
		"エラーを検出したため終了します",
		"error detected, exiting",
		"app.flow.error_exit",
	)
	register(
		"检测到错误，按回车重试",
		"エラーを検出しました。Enter で再試行します",
		"error detected, press Enter to retry",
		"app.flow.retry_prompt",
	)
	register(
		"重新开始",
		"再試行を開始します",
		"retrying",
		"app.flow.retry_start",
	)
	register(
		"生成 JSON 失败：%v",
		"JSON の生成に失敗しました: %v",
		"failed to generate JSON: %v",
		"app.flow.json_failed",
	)

	// ---------------------------------------------------------------- 歌手列表选择
	register(
		"专辑",
		"アルバム",
		"albums",
		"app.flow.kind_albums",
	)
	register(
		"音乐视频",
		"ミュージックビデオ",
		"music videos",
		"app.flow.kind_music_videos",
	)
	register(
		"单曲",
		"楽曲",
		"songs",
		"app.flow.kind_songs",
	)
	register(
		"歌手",
		"アーティスト",
		"artists",
		"app.flow.kind_artists",
	)
	register(
		"专辑名",
		"アルバム名",
		"album name",
		"app.flow.col_album_name",
	)
	register(
		"MV 名",
		"MV 名",
		"mv name",
		"app.flow.col_mv_name",
	)
	register(
		"日期",
		"日付",
		"date",
		"app.flow.col_date",
	)
	register(
		"ID",
		"ID",
		"id",
		"app.flow.col_id",
	)
	register(
		"已选择全部选项",
		"すべての項目を選択しました",
		"selected all options",
		"app.flow.select_all_done",
	)
	register(
		"请从上方 %s 列表中选择（多选用逗号分隔，支持区间，输入 all 全选）",
		"上記の%sから選択してください（複数はカンマ区切り、範囲指定可、all で全選択）",
		"select from the %s above (comma separated, ranges supported, type all to select everything)",
		"app.flow.select_hint",
	)
	register(
		"输入选项：",
		"選択を入力: ",
		"enter your choice: ",
		"app.flow.enter_choice",
	)
	register(
		"已选择以下选项",
		"以下の項目を選択しました",
		"selected the following options",
		"app.flow.selected_options",
	)
	register(
		"无效选项：%s",
		"無効な項目です: %s",
		"invalid option: %s",
		"app.flow.option_invalid",
	)
	register(
		"选项超出范围：%s",
		"項目が範囲外です: %s",
		"option out of range: %s",
		"app.flow.option_out_of_range",
	)
	register(
		"区间无效：%v",
		"範囲指定が無効です: %v",
		"invalid range: %v",
		"app.flow.range_invalid",
	)
	register(
		"区间超出范围：%v",
		"範囲が範囲外です: %v",
		"range out of range: %v",
		"app.flow.range_out_of_range",
	)

	// ---------------------------------------------------------------- 搜索与音质
	register(
		"搜索%s：%s（商店 %s）",
		"%s を検索: %s（ストアフロント %s）",
		"searching %s: %s in storefront %s",
		"app.flow.search_start",
	)
	register(
		"搜索流程失败：%v",
		"検索処理に失敗しました: %v",
		"search failed: %v",
		"app.flow.search_failed",
	)
	register(
		"--search 需要提供关键词",
		"--search にはクエリが必要です",
		"--search requires a query",
		"app.flow.search_query_required",
	)
	register(
		"没有结果",
		"結果がありません",
		"no results found",
		"app.flow.no_results",
	)
	register(
		"已取消选择",
		"選択をキャンセルしました",
		"selection cancelled",
		"app.flow.selection_cancelled",
	)
	register(
		"方向键移动，回车选择",
		"矢印キーで移動し Enter で選択",
		"use arrow keys to move, Enter to select",
		"app.flow.navigate_prompt",
	)
	register(
		"⬅️  上一页",
		"⬅️  前のページ",
		"⬅️  previous page",
		"app.flow.page_prev",
	)
	register(
		"➡️  下一页",
		"➡️  次のページ",
		"➡️  next page",
		"app.flow.page_next",
	)
	register(
		"%d 首",
		"%d 曲",
		"%d tracks",
		"app.flow.track_count",
	)
	register(
		"%s - %s（%s，%s）",
		"%s - %s（%s、%s）",
		"%s - %s (%s, %s)",
		"app.flow.search_album_line",
	)
	register(
		"%s - %s（%s）",
		"%s - %s（%s）",
		"%s - %s (%s)",
		"app.flow.search_song_line",
	)
	register(
		"%s（%s）",
		"%s（%s）",
		"%s (%s)",
		"app.flow.search_artist_line",
	)
	register(
		"已选择歌手，列出全部专辑与 MV",
		"アーティストを選択しました。アルバムと MV を一覧します",
		"artist selected, listing all albums and videos",
		"app.flow.artist_selected",
	)
	register(
		"获取 %s 的可用音质",
		"%s の利用可能な音質を取得します",
		"fetching available qualities for %s",
		"app.flow.fetch_qualities",
	)
	register(
		"选择下载音质",
		"ダウンロード音質を選択",
		"select a quality to download",
		"app.flow.quality_prompt",
	)
	register(
		"无损（ALAC）",
		"ロスレス（ALAC）",
		"lossless (ALAC)",
		"app.flow.quality_desc_alac",
	)
	register(
		"高品质（AAC）",
		"高音質（AAC）",
		"high quality (AAC)",
		"app.flow.quality_desc_aac",
	)
	register(
		"Dolby Atmos",
		"Dolby Atmos",
		"Dolby Atmos",
		"app.flow.quality_desc_atmos",
	)
	register(
		"音质设为 Dolby Atmos",
		"音質を Dolby Atmos に設定しました",
		"quality set to Dolby Atmos",
		"app.flow.quality_atmos",
	)
	register(
		"音质设为 AAC",
		"音質を AAC に設定しました",
		"quality set to AAC",
		"app.flow.quality_aac",
	)
	register(
		"音质设为 ALAC 无损",
		"音質を ALAC ロスレスに設定しました",
		"quality set to ALAC lossless",
		"app.flow.quality_alac",
	)
}
