package fairplayrip

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"amdl/internal/config"

	"github.com/grafov/m3u8"
	"github.com/itouakirai/mp4ff/mp4"
)

// applePlaylist 造一份 Apple Music 风格的 byterange 播放列表：EXT-X-MAP 指向
// init 段，之后每段一个 EXT-X-KEY + EXT-X-BYTERANGE，所有分片都落在同一个
// track.mp4 上。
func applePlaylist(count int, initLen, segLen int64, mediaSequence int) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:7\n")
	b.WriteString("#EXT-X-TARGETDURATION:10\n")
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", mediaSequence)
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"track.mp4\",BYTERANGE=\"%d@0\"\n", initLen)
	for i := 0; i < count; i++ {
		b.WriteString("#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://key\",KEYFORMAT=\"com.apple.streamingkeydelivery\",KEYFORMATVERSIONS=\"1\"\n")
		b.WriteString("#EXTINF:9.6,\n")
		fmt.Fprintf(&b, "#EXT-X-BYTERANGE:%d@%d\n", segLen, initLen+int64(i)*segLen)
		b.WriteString("track.mp4\n")
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

func decodePlaylist(t *testing.T, text string) *m3u8.MediaPlaylist {
	t.Helper()
	p, listType, err := m3u8.Decode(*bytes.NewBufferString(text), true)
	if err != nil {
		t.Fatalf("decode playlist: %v", err)
	}
	if listType != m3u8.MEDIA {
		t.Fatalf("list type = %v, want MEDIA", listType)
	}
	return p.(*m3u8.MediaPlaylist)
}

// TestPlaylistSegmentsUsesCountNotSliceLen 是用户报的那个 bug 的回归测试：
// 7 段的播放列表必须给出 7 个分片，而不是 1024。
func TestPlaylistSegmentsUsesCountNotSliceLen(t *testing.T) {
	const n = 7
	text := applePlaylist(n, 512, 1000, 1)

	p := decodePlaylist(t, text)
	// 这就是坑的现场：Segments 是 1024 格的环形缓冲区。
	if got := len(p.Segments); got != 1024 {
		t.Fatalf("len(Segments) = %d, 期望 1024（m3u8 的环形缓冲区容量）", got)
	}
	if got := int(p.Count()); got != n {
		t.Fatalf("Count() = %d, 期望 %d", got, n)
	}

	segs := playlistSegments(p)
	if len(segs) != n {
		t.Fatalf("playlistSegments() 返回 %d 段，期望 %d 段（不是 1024）", len(segs), n)
	}
	for i, seg := range segs {
		if seg == nil {
			t.Fatalf("第 %d 段是 nil", i)
		}
		wantSeq := uint64(1 + i) // EXT-X-MEDIA-SEQUENCE:1
		if seg.SeqId != wantSeq {
			t.Errorf("第 %d 段 SeqId = %d, 期望 %d", i, seg.SeqId, wantSeq)
		}
		if seg.URI != "track.mp4" {
			t.Errorf("第 %d 段 URI = %q", i, seg.URI)
		}
		if seg.Limit != 1000 {
			t.Errorf("第 %d 段 Limit = %d, 期望 1000", i, seg.Limit)
		}
		if want := int64(512 + int64(i)*1000); seg.Offset != want {
			t.Errorf("第 %d 段 Offset = %d, 期望 %d", i, seg.Offset, want)
		}
		if seg.Key == nil || seg.Key.URI != "skd://key" {
			t.Errorf("第 %d 段没有继承 EXT-X-KEY", i)
		}
	}
}

// TestParseMediaPlaylistReturnsRealSegments 走完整解析路径（含 key 过滤），
// 确认修复落到实处。
func TestParseMediaPlaylistReturnsRealSegments(t *testing.T) {
	const n = 7
	text := applePlaylist(n, 512, 1000, 1)

	segs, err := parseMediaPlaylist(io.NopCloser(strings.NewReader(text)))
	if err != nil {
		t.Fatalf("parseMediaPlaylist: %v", err)
	}
	if len(segs) != n {
		t.Fatalf("parseMediaPlaylist 返回 %d 段, 期望 %d", len(segs), n)
	}
	if err := checkFragmentCount(len(segs), n); err != nil {
		t.Fatalf("齐全的流不该报错: %v", err)
	}
	if err := checkFragmentCount(len(segs), n-1); err == nil {
		t.Fatal("少一个分片时必须报错")
	}
}

// TestPlaylistSegmentsHandlesWrappedRingBuffer 覆盖滑动播放列表：写完一圈之后
// 写入位置回到数组开头，被淘汰的槽位留空，SeqId 也不从 0 开始。返回顺序必须
// 按 SeqId，而不是数组下标。
func TestPlaylistSegmentsHandlesWrappedRingBuffer(t *testing.T) {
	p, err := m3u8.NewMediaPlaylist(8, 8)
	if err != nil {
		t.Fatalf("NewMediaPlaylist: %v", err)
	}
	p.SeqNo = 101
	for i := 0; i < 8; i++ {
		if err := p.AppendSegment(&m3u8.MediaSegment{
			URI:   fmt.Sprintf("f%d.m4s", i),
			Limit: 100,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	// 滑掉最旧的三段，槽位留空（滑动窗口淘汰）。
	for i := 0; i < 3; i++ {
		if err := p.Remove(); err != nil {
			t.Fatalf("remove: %v", err)
		}
	}
	p.Segments[0], p.Segments[1], p.Segments[2] = nil, nil, nil
	// 再追加两段：写入位置回绕到数组开头，数组顺序与 SeqId 顺序就此错开。
	for i := 0; i < 2; i++ {
		if err := p.AppendSegment(&m3u8.MediaSegment{
			URI:   fmt.Sprintf("f%d.m4s", 8+i),
			Limit: 100,
		}); err != nil {
			t.Fatalf("append wrapped %d: %v", i, err)
		}
	}
	if got := int(p.Count()); got != 7 {
		t.Fatalf("Count() = %d, 期望 7", got)
	}
	if p.Segments[2] != nil {
		t.Fatalf("这个用例需要第 2 个槽位是空的")
	}
	// 回绕后的数组顺序里，最后一格比第一格更旧，排序是必须的。
	if p.Segments[0] == nil || p.Segments[7] == nil || p.Segments[0].SeqId < p.Segments[7].SeqId {
		t.Fatalf("环形缓冲区的回绕布局没造出来")
	}

	segs := playlistSegments(p)
	if len(segs) != 7 {
		t.Fatalf("playlistSegments() 返回 %d 段, 期望 7", len(segs))
	}
	for i, seg := range segs {
		if seg == nil {
			t.Fatalf("第 %d 段是 nil（空槽必须被过滤掉）", i)
		}
		want := uint64(104 + i) // SeqId 从 101 起算，滑掉 101..103
		if seg.SeqId != want {
			t.Errorf("第 %d 段 SeqId = %d, 期望 %d（应按 SeqId 排序）", i, seg.SeqId, want)
		}
	}
	for i := 1; i < len(segs); i++ {
		if segs[i-1].SeqId >= segs[i].SeqId {
			t.Fatalf("分片没有按 SeqId 升序: %d 后面跟着 %d", segs[i-1].SeqId, segs[i].SeqId)
		}
	}
}

// TestPlaylistByteRangeSumMatchesStream 校验字节范围算术：各段 byterange 之和
// 必须等于 init 段之后实际要读的字节数。
func TestPlaylistByteRangeSumMatchesStream(t *testing.T) {
	const (
		n      = 7
		initLn = int64(512)
		segLn  = int64(1000)
	)
	text := applePlaylist(n, initLn, segLn, 1)
	segs := playlistSegments(decodePlaylist(t, text))

	want := int64(n) * segLn
	if got := expectedByteRangeTotal(segs); got != want {
		t.Fatalf("expectedByteRangeTotal() = %d, 期望 %d", got, want)
	}

	// 流的总长 = init 段 + 各分片范围之和，而且分片范围首尾相接。
	streamLen := initLn + want
	var cursor int64 = initLn
	for i, seg := range segs {
		if seg.Offset != cursor {
			t.Fatalf("第 %d 段 Offset = %d, 期望 %d（范围必须首尾相接）", i, seg.Offset, cursor)
		}
		cursor += seg.Limit
	}
	if cursor != streamLen {
		t.Fatalf("分片范围读到头是 %d, 期望 %d", cursor, streamLen)
	}

	// 没有 byterange 的分片不参与求和。
	segs = append(segs, &m3u8.MediaSegment{URI: "track.mp4"})
	if got := expectedByteRangeTotal(segs); got != want {
		t.Fatalf("加上无范围的分片后合计 = %d, 期望仍是 %d", got, want)
	}
	// 空列表 / nil 播放列表都不该炸。
	if got := expectedByteRangeTotal(nil); got != 0 {
		t.Fatalf("空列表合计 = %d, 期望 0", got)
	}
	if segs := playlistSegments(nil); segs != nil {
		t.Fatalf("nil 播放列表应返回 nil, 得到 %v", segs)
	}
}

// TestCheckFragmentCountMessages 锁住 app/rip.go 的重试逻辑要匹配的三条英文片段。
func TestCheckFragmentCountMessages(t *testing.T) {
	cases := []struct {
		name     string
		expected int
		decoded  int
		wantSub  []string
		isNil    bool
	}{
		{name: "齐全", expected: 7, decoded: 7, isNil: true},
		{name: "多个分片无害", expected: 7, decoded: 8, isNil: true},
		{name: "一个分片都没有", expected: 7, decoded: 0, wantSub: []string{"without a single media fragment"}},
		{name: "少了解码出来的", expected: 7, decoded: 5, wantSub: []string{"incomplete track", "fragment(s), only", "7", "5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkFragmentCount(tc.expected, tc.decoded)
			if tc.isNil {
				if err != nil {
					t.Fatalf("不该报错: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("应当报错")
			}
			for _, sub := range tc.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("错误信息 %q 缺少 %q", err.Error(), sub)
				}
			}
		})
	}
}

// TestDownloadAndDecryptFileWithoutMediaFragment 跑一遍真实流水线：流里只有
// init 段（ftyp+moov）就结束，必须给出 "without a single media fragment"。
func TestDownloadAndDecryptFileWithoutMediaFragment(t *testing.T) {
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(44100, "audio", "und")
	var stream bytes.Buffer
	if err := init.Encode(&stream); err != nil {
		t.Fatalf("encode init: %v", err)
	}

	segments := playlistSegments(decodePlaylist(t, applePlaylist(7, 512, 1000, 1)))
	outfile := t.TempDir() + "/track.mp4"
	cfg := config.ConfigSet{General: config.GeneralConfig{MaxMemoryLimit: 256}}

	err := downloadAndDecryptFile("http://127.0.0.1:1", bytes.NewReader(stream.Bytes()), outfile, "1",
		segments, int64(stream.Len()), cfg)
	if err == nil {
		t.Fatal("只有 init 段时必须报错")
	}
	if !errors.Is(err, errNoMediaFragment) {
		t.Fatalf("err = %v, 期望 errNoMediaFragment", err)
	}
	if !strings.Contains(err.Error(), "without a single media fragment") {
		t.Fatalf("错误信息 %q 不含规定片段", err.Error())
	}
	if _, statErr := os.Stat(outfile); statErr == nil {
		t.Fatal("失败的下载不该留下输出文件")
	}
}
