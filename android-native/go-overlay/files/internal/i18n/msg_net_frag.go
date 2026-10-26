package i18n

// fairplay-rip（逐片下载 + 解密）用到的文案。
// 区固定为 net，键名都是 net.frag.*。
func init() {
	// 进度条描述。进度条本身不走 logx（它靠 \r 重画一行），描述文字在这里取。
	register(
		"解密中…",
		"復号中…",
		"decrypting…",
		"net.frag.progress_decrypting",
	)

	// 一条流解密并写出完成。
	register(
		"解密完成",
		"復号完了",
		"decrypted",
		"net.frag.decrypted",
	)

	// init 段没能完全清理（少见的 stsd 布局），还能继续，只是提醒。
	register(
		"init 段清理不完整：%v",
		"init セグメントの整形が不完全です: %v",
		"could not fully sanitize the init segment: %v",
		"net.frag.sanitize_init",
	)

	// 播放列表里的分片数（--verbose）。
	register(
		"播放列表共 %d 个分片",
		"プレイリストのフラグメント数: %d",
		"playlist lists %d fragment(s)",
		"net.frag.playlist_fragments",
	)

	// 实际解码出的分片数（--verbose）。
	register(
		"已解码 %d 个分片（播放列表 %d 个）",
		"%d 個のフラグメントを復号しました（プレイリスト %d 個）",
		"decoded %d fragment(s) of %d",
		"net.frag.decoded_fragments",
	)

	// 连接中途断开，用 Range 从偏移处续传（可恢复，所以是警告级）。
	register(
		"连接中断，从偏移 %d 处续传（第 %d 次重连）",
		"接続が切れたため、オフセット %d から再開します（%d 回目の再接続）",
		"stream interrupted, resuming at offset %d (reconnect %d)",
		"net.frag.resume",
	)

	// 从流里读到的字节数（--verbose）。
	register(
		"读取 %s",
		"%s を読み込みました",
		"read %s",
		"net.frag.read_bytes",
	)

	// 分片字节范围之和与实际读到的字节数一致（--verbose）。
	register(
		"字节范围合计 %s，与实际读取一致",
		"バイト範囲の合計 %s は実際の読み込み量と一致します",
		"byte ranges add up to %s, matching the bytes read",
		"net.frag.byterange_total",
	)

	// 分片字节范围之和与实际读到的字节数不一致（--verbose，说明列表和流不同步）。
	register(
		"字节范围合计 %d 字节，实际读取 %d 字节",
		"バイト範囲の合計 %d バイト、実際の読み込みは %d バイト",
		"byte ranges add up to %d byte(s) but %d were read",
		"net.frag.byterange_mismatch",
	)
}
