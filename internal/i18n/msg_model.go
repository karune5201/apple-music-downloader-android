package i18n

// 本文件是 model 区的文案：交互式选择专辑/歌单/电台曲目时的提示与校验。
//
// 约定：曲目表本身（编号、曲名、列、颜色）属于数据，不走这里；
// 只有表格前后的说明文字、以及等待 stdin 时的提示才翻译。
func init() {
	// 表格下方的用法说明。
	register(
		"请从上方曲目中选择（可用逗号分隔多个，支持范围，输入 all 选择全部）",
		"上の曲目から選択してください（カンマ区切りで複数指定可、範囲指定可、all で全選択）",
		"select from the tracks above (comma separated, ranges allowed, or 'all' for everything)",
		"model.select.hint",
	)

	// 读取 stdin 前的可见提示。英文结尾保留 "Enter your choice: "，
	// 兼容只认后缀的旧版 App。
	register(
		"请输入选项：",
		"番号を入力してください：",
		"Enter your choice: ",
		"model.select.prompt",
	)

	// 读 stdin 失败（后面仍按空输入继续）。
	register(
		"读取输入失败：%v",
		"入力の読み取りに失敗しました：%v",
		"failed reading input: %v",
		"model.select.read_failed",
	)

	// 输入 all。
	register(
		"已选择全部曲目",
		"すべての曲目を選択しました",
		"selected all tracks",
		"model.select.all_selected",
	)

	// 单个选项不是数字。
	register(
		"无效的选项：%v",
		"無効なオプションです：%v",
		"invalid option: %v",
		"model.select.invalid_option",
	)

	// 单个选项超出曲目数量。
	register(
		"选项超出范围：%v",
		"オプションが範囲外です：%v",
		"option out of range: %v",
		"model.select.out_of_range",
	)

	// 范围写法不是数字。
	register(
		"无效的范围：%v",
		"無効な範囲です：%v",
		"invalid range: %v",
		"model.select.invalid_range",
	)

	// 范围越界或起止颠倒。
	register(
		"范围超出范围：%v",
		"範囲が範囲外です：%v",
		"range out of range: %v",
		"model.select.range_out_of_range",
	)

	// 电台取单曲所属专辑失败，跳过这一首。
	register(
		"获取专辑信息失败：%v",
		"アルバム情報の取得に失敗しました：%v",
		"failed getting album response: %v",
		"model.station.album_resp_failed",
	)
}
