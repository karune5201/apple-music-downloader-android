package fairplayrip

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"

	"amdl/internal/config"
	"amdl/internal/download"
	"amdl/internal/i18n"
	"amdl/internal/logx"
	"amdl/internal/temari"
	"amdl/internal/wrapper"

	"github.com/grafov/m3u8"
	"github.com/itouakirai/mp4ff/mp4"
	"github.com/schollz/progressbar/v3"
)

// 本包不当超时警察：上游卡住就让它卡着，这是产品决定。连接中途断掉则由
// resumeReader 用 Range 续传（最多 maxStreamRetries 次）。
var (
	// errNoMediaFragment 表示流里只有 init 段，一个媒体分片都没有。
	errNoMediaFragment = errors.New("stream ended after the init segment without a single media fragment")
	// errNoMediaSegment 表示播放列表里一段都没有。
	errNoMediaSegment = errors.New("no segments extracted from playlist")
)

// checkFragmentCount 把播放列表声明的分片数与真正解码出来的分片数对上。
//
// 列表里说有几段，流里就应该有几段：少了说明流被截断或服务端少给了数据，
// 直接失败好过写出一条不完整的歌。
func checkFragmentCount(expected, decoded int) error {
	if decoded == 0 {
		return errNoMediaFragment
	}
	if decoded < expected {
		return fmt.Errorf("incomplete track: playlist lists %d fragment(s), only %d were decoded", expected, decoded)
	}
	return nil
}

// fetchTemplate obtains the decryption template for adamId/uri from lite-server
// and parses the 40020-style JSON body with the pure-Go Temari port.
func fetchTemplate(baseURL, adam, uri string) (*temari.Template, error) {
	body, err := wrapper.GetKeyTemplateJSON(baseURL, adam, uri)
	if err != nil {
		return nil, err
	}
	return temari.FromJSON(body)
}

// maxStreamRetries 是一条流断开之后最多重新连接几次。
const maxStreamRetries = 5

// resumeReader 把一条 HTTP 响应体当成可续传的流。
//
// 连接中途被掐断时（长连接到期、CDN 掉线），它用 Range: bytes=N- 重新请求
// 同一个 URL，从已交付的偏移继续往下读，调用方看到的是连续的一条流。服务端
// 不认识 Range、干脆整个文件重发时，它自己把前面那 N 个字节吃掉。重试次数
// 用尽就报错，并把中断的偏移带上。
//
// 这里没有任何超时：上游卡住就让它卡着。
type resumeReader struct {
	url    string
	header http.Header

	body       io.ReadCloser
	total      int64 // 首次响应给出的总长度，未知时 -1
	offset     int64 // 已经交付给调用方的字节数
	reconnects int   // 已经重连过的次数
	cause      error // 最近一次断流的原因
	done       bool  // 是否已经读到结尾
}

func newResumeReader(rawURL string, header http.Header) (*resumeReader, error) {
	r := &resumeReader{url: rawURL, header: header, total: -1}
	if err := r.connect(); err != nil {
		return nil, err
	}
	return r, nil
}

// Total 是这条流的总长度，服务端没给 Content-Length 时返回 -1。
func (r *resumeReader) Total() int64 { return r.total }

// Offset 是已经读出来的字节数，也就是下一次续传的起点。
func (r *resumeReader) Offset() int64 { return r.offset }

// connect 按当前偏移打开（或重新打开）响应体。
func (r *resumeReader) connect() error {
	req, err := http.NewRequest("GET", r.url, nil)
	if err != nil {
		return err
	}
	if r.header != nil {
		req.Header = r.header.Clone()
	}
	if r.offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", r.offset))
	}
	resp, err := download.Client.Do(req)
	if err != nil {
		return err
	}
	switch {
	case r.offset == 0 || resp.StatusCode == http.StatusPartialContent:
		// 响应体正好从 r.offset 开始
	case resp.StatusCode == http.StatusOK:
		// 服务端忽略了 Range，整个文件重发：自己跳过已经拿到的那一段
		if _, err := io.CopyN(io.Discard, resp.Body, r.offset); err != nil {
			resp.Body.Close()
			return fmt.Errorf("skip already-read %d byte(s) after range-less replay: %w", r.offset, err)
		}
	default:
		resp.Body.Close()
		return fmt.Errorf("resume at offset %d: unexpected HTTP status %s", r.offset, resp.Status)
	}
	if r.total < 0 && resp.ContentLength >= 0 {
		r.total = r.offset + resp.ContentLength
	}
	r.body = resp.Body
	return nil
}

func (r *resumeReader) Read(p []byte) (int, error) {
	for {
		if r.done {
			return 0, io.EOF
		}
		if r.body == nil {
			if r.reconnects >= maxStreamRetries {
				return 0, fmt.Errorf("stream interrupted at offset %d: %d reconnect attempt(s) exhausted: %v",
					r.offset, r.reconnects, r.cause)
			}
			r.reconnects++
			if err := r.connect(); err != nil {
				r.cause = err
				return 0, fmt.Errorf("reconnect at offset %d failed (%d/%d): %v",
					r.offset, r.reconnects, maxStreamRetries, err)
			}
			logx.WarnT("net.frag.resume", r.offset, r.reconnects)
		}
		n, err := r.body.Read(p)
		r.offset += int64(n)
		if err == nil {
			return n, nil
		}
		if errors.Is(err, io.EOF) {
			r.done = true
			r.closeBody()
			return n, io.EOF
		}
		// 断流：丢掉这一路响应，下次 Read 用 Range 续上
		r.cause = err
		r.closeBody()
		if n > 0 {
			return n, nil
		}
	}
}

func (r *resumeReader) closeBody() {
	body := r.body
	r.body = nil
	if body != nil {
		body.Close()
	}
}

// Close 关掉当前响应体，可以重复调用。
func (r *resumeReader) Close() error {
	r.done = true
	r.closeBody()
	return nil
}

type decryptJob struct {
	Seq       int              // fragment sequence number, used for ordering
	Frag      *mp4.Fragment    // original fragment
	Tmpl      *temari.Template // key template
	RawOffset int64
}

type decryptResult struct {
	Seq       int
	Frag      *mp4.Fragment
	RawOffset int64
}

// Run streams fragmented MP4 and decrypts on the fly: HTTP body is fed directly
// through a fragment-reader -> decrypt-workers -> in-order-writer pipeline.
func Run(adamId string, playlistUrl string, outfile string, Config config.ConfigSet) error {
	if Config.General.LiteServer == "" {
		return errors.New("lite-server is not configured in config.yaml")
	}
	header := make(http.Header)

	req, err := http.NewRequest("GET", playlistUrl, nil)
	if err != nil {
		return err
	}
	req.Header = header
	do, err := download.Client.Do(req)
	if err != nil {
		return err
	}

	segments, err := parseMediaPlaylist(do.Body)
	if err != nil {
		return err
	}
	// len(segments) 才是播放列表真正的分片数；别再拿播放列表结构的 Segments
	// 切片长度当分片数，那是环形缓冲区的容量（见 playlistSegments）。
	if len(segments) == 0 {
		return errNoMediaSegment
	}
	playlistCount := len(segments)
	logx.DebugT("net.frag.playlist_fragments", playlistCount)

	segment := segments[0]
	if segment.Limit <= 0 {
		return errors.New("non-byterange playlists are currently unsupported")
	}

	parsedUrl, err := url.Parse(playlistUrl)
	if err != nil {
		return err
	}
	fileUrl, err := parsedUrl.Parse(segment.URI)
	if err != nil {
		return err
	}

	// 媒体流完全不设超时；断了由 resumeReader 用 Range 续上。
	stream, err := newResumeReader(fileUrl.String(), header)
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := downloadAndDecryptFile(Config.General.LiteServer, stream, outfile, adamId, segments, stream.Total(), Config); err != nil {
		return err
	}
	logx.OKT("net.frag.decrypted")
	return nil
}

func downloadAndDecryptFile(liteServer string, in io.Reader, outfile string,
	adamId string, segs []*m3u8.MediaSegment, totalLen int64, Config config.ConfigSet) error {
	var buffer bytes.Buffer
	var outBuf *bufio.Writer
	var outFile *os.File
	var tmpPath string
	var err error
	MaxMemorySize := int64(Config.General.MaxMemoryLimit * 1024 * 1024)
	inBuf := bufio.NewReader(in)

	// 没有 Content-Length 时（分块传输、续传）用播放列表的字节范围估总长，
	// 否则 -1 会被当成"很小"而把整首歌塞进内存。
	if totalLen < 0 {
		totalLen = expectedByteRangeTotal(segs)
	}

	if totalLen <= MaxMemorySize {
		outBuf = bufio.NewWriter(&buffer)
	} else {
		// Stream to a temp file first and rename into place on success, so a
		// failed download never leaves a truncated track at the final path.
		tmpPath = outfile + ".part"
		outFile, err = os.Create(tmpPath)
		if err != nil {
			return err
		}
		outBuf = bufio.NewWriter(outFile)
	}
	defer func() {
		if outFile != nil {
			outFile.Close()
			os.Remove(tmpPath)
		}
	}()
	init, offset, err := ReadInitSegment(inBuf)
	if err != nil {
		return err
	}
	if init == nil {
		return errors.New("no init segment found")
	}

	tracks, err := TransformInit(init)
	if err != nil {
		return err
	}
	err = sanitizeInit(init)
	if err != nil {
		logx.WarnT("net.frag.sanitize_init", err)
	}
	err = init.Encode(outBuf)
	if err != nil {
		return err
	}

	bar := progressbar.NewOptions64(totalLen,
		progressbar.OptionSetElapsedTime(false),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionShowCount(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription(i18n.T("net.frag.progress_decrypting")),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "",
			SaucerHead:    "",
			SaucerPadding: "",
			BarStart:      "",
			BarEnd:        "",
		}),
	)
	bar.Add64(int64(offset))

	eg, ctx := errgroup.WithContext(context.Background())

	// 真正读出来并交给解密线程的分片数。只由下面的读取协程写，eg.Wait() 之后
	// 才读，靠 errgroup 的 Wait 保证可见性。
	decodedFragments := 0

	jobs := make(chan *decryptJob, 10)
	results := make(chan *decryptResult, 10)

	eg.Go(func() error {
		buffer := make(map[int]*decryptResult)
		expectedSeq := 0
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case res, ok := <-results:
				if !ok {
					return nil
				}
				buffer[res.Seq] = res
				for {
					if readyRes, exists := buffer[expectedSeq]; exists {
						if err := readyRes.Frag.Encode(outBuf); err != nil {
							return fmt.Errorf("encode fragment seq %d failed: %w", expectedSeq, err)
						}
						bar.Add64(readyRes.RawOffset)
						delete(buffer, expectedSeq)
						expectedSeq++
					} else {
						break
					}
				}
			}
		}
	})

	var workerWg sync.WaitGroup
	for i := 0; i < 10; i++ {
		workerWg.Add(1)
		eg.Go(func() error {
			defer workerWg.Done()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case job, ok := <-jobs:
					if !ok {
						return nil
					}
					if err := DecryptFragment(job.Frag, tracks, job.Tmpl); err != nil {
						return fmt.Errorf("decryptFragment seq %d: %w", job.Seq, err)
					}
					select {
					case results <- &decryptResult{
						Seq:       job.Seq,
						Frag:      job.Frag,
						RawOffset: job.RawOffset,
					}:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		})
	}

	eg.Go(func() error {
		workerWg.Wait()
		close(results)
		return nil
	})

	eg.Go(func() error {
		defer close(jobs)
		seq := 0
		var tmpl *temari.Template
		var tmplURI string
		tmplCache := map[string]*temari.Template{}

		for i := 0; ; i++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			var frag *mp4.Fragment
			rawoffset := offset
			frag, offset, err = ReadNextFragment(inBuf, offset)
			rawoffset = offset - rawoffset
			if err != nil {
				return fmt.Errorf("read fragment: %w", err)
			}
			if frag == nil {
				break
			}

			if i >= len(segs) {
				return fmt.Errorf("segment number out of sync: fragment %d, playlist lists %d", i, len(segs))
			}
			segment := segs[i]
			if segment == nil {
				return fmt.Errorf("segment number out of sync: fragment %d, playlist lists %d", i, len(segs))
			}

			// The key may switch at any segment (not only the first or second),
			// so follow every EXT-X-KEY change and reuse templates already fetched.
			key := segment.Key
			if key != nil && key.URI != tmplURI {
				if cached, ok := tmplCache[key.URI]; ok {
					tmpl = cached
				} else {
					if key.URI == wrapper.PrefetchKey {
						tmpl, err = fetchTemplate(liteServer, "0", key.URI)
					} else {
						tmpl, err = fetchTemplate(liteServer, adamId, key.URI)
					}
					if err != nil {
						return err
					}
					tmplCache[key.URI] = tmpl
				}
				tmplURI = key.URI
			}

			job := &decryptJob{
				Seq:       seq,
				Frag:      frag,
				Tmpl:      tmpl,
				RawOffset: int64(rawoffset),
			}

			select {
			case jobs <- job:
			case <-ctx.Done():
				return ctx.Err()
			}
			seq++
		}
		decodedFragments = seq
		return nil
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	// 播放列表说有几段就得有几段：少了说明流被截断，一个分片都没有说明流里
	// 只有 init 段。两种都不该写成一条"能播但其实不完整"的歌。
	if err := checkFragmentCount(len(segs), decodedFragments); err != nil {
		return err
	}
	logx.DebugT("net.frag.decoded_fragments", decodedFragments, len(segs))
	if want := expectedByteRangeTotal(segs); want > 0 {
		if got := int64(offset); got != want {
			logx.DebugT("net.frag.byterange_mismatch", int64(want), got)
		} else {
			logx.DebugT("net.frag.byterange_total", want)
		}
	}

	err = outBuf.Flush()
	if err != nil {
		return err
	}
	if totalLen <= MaxMemorySize {
		ofh, err := os.Create(outfile)
		if err != nil {
			return err
		}
		defer ofh.Close()

		_, err = ofh.Write(buffer.Bytes())
		if err != nil {
			return err
		}
	} else {
		// Commit: close the temp file and atomically move it to the final name.
		if err := outFile.Close(); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, outfile); err != nil {
			os.Remove(tmpPath)
			return err
		}
		outFile = nil
		tmpPath = ""
	}
	logx.DebugT("net.frag.read_bytes", logx.Bytes(int64(offset)))
	return nil
}

// --- Sanitize / helpers -------------------------------------------------------

func sanitizeInit(init *mp4.InitSegment) error {
	traks := init.Moov.Traks
	if len(traks) > 1 {
		return errors.New("more than 1 track found")
	}
	stsd := traks[0].Mdia.Minf.Stbl.Stsd
	// 0 或 1 条 stsd 记录都没什么可删的（0 只出现在空 trak / 测试构造的 init）。
	if stsd.SampleCount <= 1 {
		return nil
	}
	if stsd.SampleCount > 2 {
		return fmt.Errorf("expected only 1 or 2 entries in stsd, got %d", stsd.SampleCount)
	}
	children := stsd.Children
	if children[0].Type() != children[1].Type() {
		return errors.New("children in stsd are not of the same type")
	}
	stsd.Children = children[:1]
	stsd.SampleCount = 1
	return nil
}

func filterResponse(f io.Reader) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	scanner := bufio.NewScanner(f)
	prefix := []byte("#EXT-X-KEY:")
	keyFormat := []byte("streamingkeydelivery")
	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if bytes.HasPrefix(lineBytes, prefix) && !bytes.Contains(lineBytes, keyFormat) {
			continue
		}
		_, err := buf.Write(lineBytes)
		if err != nil {
			return nil, err
		}
		_, err = buf.WriteString("\n")
		if err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}

// playlistSegments 返回播放列表里真正的分片，顺序按 SeqId。
//
// m3u8 的解码器用 NewMediaPlaylist(8, 1024) 建表，MediaPlaylist.Segments 是
// 一个 1024 格的环形缓冲区：len(Segments) 永远等于容量 1024，只有 Count()
// 知道里面真有几个分片，而且环形表的空槽、滑动播放列表回绕后的顺序都得自己
// 收拾。把 len(Segments) 当分片数，每一首都会以
// "playlist=1024 fragments=7" 的形态失败。
//
// 按 SeqId 排序是为了滑动播放列表：回绕之后数组下标的顺序和播放顺序不一致。
func playlistSegments(p *m3u8.MediaPlaylist) []*m3u8.MediaSegment {
	if p == nil {
		return nil
	}
	out := make([]*m3u8.MediaSegment, 0, int(p.Count()))
	for _, seg := range p.Segments {
		if seg != nil {
			out = append(out, seg)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SeqId < out[j].SeqId })
	return out
}

// expectedByteRangeTotal 把各分片的 EXT-X-BYTERANGE 长度加起来，这才是这条流
// 应该有的字节数。没有 Range 的分片按 0 计，不参与求和。
func expectedByteRangeTotal(segs []*m3u8.MediaSegment) int64 {
	var total int64
	for _, seg := range segs {
		if seg == nil || seg.Limit <= 0 {
			continue
		}
		total += seg.Limit
	}
	return total
}

// parseMediaPlaylist 读播放列表并返回真实分片（见 playlistSegments）。
func parseMediaPlaylist(r io.ReadCloser) ([]*m3u8.MediaSegment, error) {
	defer r.Close()
	playlistBuf, err := filterResponse(r)
	if err != nil {
		return nil, err
	}
	playlist, listType, err := m3u8.Decode(*playlistBuf, true)
	if err != nil {
		return nil, err
	}
	if listType != m3u8.MEDIA {
		return nil, errors.New("m3u8 not of media type")
	}
	mediaPlaylist := playlist.(*m3u8.MediaPlaylist)
	return playlistSegments(mediaPlaylist), nil
}

func ReadInitSegment(r io.Reader) (*mp4.InitSegment, uint64, error) {
	var offset uint64 = 0
	init := mp4.NewMP4Init()
	for i := 0; i < 2; i++ {
		box, err := mp4.DecodeBox(offset, r)
		if err != nil {
			return nil, offset, err
		}
		boxType := box.Type()
		if boxType != "ftyp" && boxType != "moov" {
			return nil, offset, fmt.Errorf("unexpected box type %s, should be ftyp or moov", boxType)
		}
		init.AddChild(box)
		offset += box.Size()
	}
	return init, offset, nil
}

func ReadNextFragment(r io.Reader, offset uint64) (*mp4.Fragment, uint64, error) {
	frag := mp4.NewFragment()
	for {
		box, err := mp4.DecodeBox(offset, r)
		if err == io.EOF {
			return nil, offset, nil
		}
		if err != nil {
			return nil, offset, err
		}
		boxType := box.Type()
		offset += box.Size()
		if boxType == "moof" || boxType == "emsg" || boxType == "prft" {
			frag.AddChild(box)
			continue
		}
		if boxType == "mdat" {
			frag.AddChild(box)
			break
		}
	}
	if frag.Moof == nil {
		return nil, offset, fmt.Errorf("more than one mdat box in fragment (box ends @ offset %d)", offset)
	}
	return frag, offset, nil
}

func FilterSbgpSgpd(children []mp4.Box) ([]mp4.Box, uint64) {
	var bytesRemoved uint64 = 0
	remainingChildren := make([]mp4.Box, 0, len(children))
	for _, child := range children {
		switch box := child.(type) {
		case *mp4.SbgpBox:
			if box.GroupingType == "seam" || box.GroupingType == "seig" {
				bytesRemoved += child.Size()
				continue
			}
		case *mp4.SgpdBox:
			if box.GroupingType == "seam" || box.GroupingType == "seig" {
				bytesRemoved += child.Size()
				continue
			}
		}
		remainingChildren = append(remainingChildren, child)
	}
	return remainingChildren, bytesRemoved
}

func TransformInit(init *mp4.InitSegment) (map[uint32]mp4.DecryptTrackInfo, error) {
	di, err := mp4.DecryptInit(init)
	tracks := make(map[uint32]mp4.DecryptTrackInfo, len(di.TrackInfos))
	for _, ti := range di.TrackInfos {
		tracks[ti.TrackID] = ti
	}
	if err != nil {
		return tracks, err
	}
	for _, trak := range init.Moov.Traks {
		stbl := trak.Mdia.Minf.Stbl
		stbl.Children, _ = FilterSbgpSgpd(stbl.Children)
	}
	return tracks, nil
}

// --- Temari-based decryption -------------------------------------------------

func cbcsDecryptRaw(data []byte, decryptBlockLen, skipBlockLen int, tmpl *temari.Template) error {
	if skipBlockLen != 0 {
		return fmt.Errorf("not full encryption of subsamples")
	}
	truncatedLen := len(data) & ^0xf
	tmpl.DecryptInto(data[:truncatedLen], data[:truncatedLen])
	return nil
}

func cbcsDecryptSample(sample []byte, subSamplePatterns []mp4.SubSamplePattern, tenc *mp4.TencBox, tmpl *temari.Template) error {
	decryptBlockLen := int(tenc.DefaultCryptByteBlock) * 16
	skipBlockLen := int(tenc.DefaultSkipByteBlock) * 16
	var pos uint32 = 0

	if len(subSamplePatterns) == 0 {
		return cbcsDecryptRaw(sample, decryptBlockLen, skipBlockLen, tmpl)
	}

	for j := 0; j < len(subSamplePatterns); j++ {
		ss := subSamplePatterns[j]
		pos += uint32(ss.BytesOfClearData)

		if ss.BytesOfProtectedData <= 0 {
			continue
		}

		err := cbcsDecryptRaw(sample[pos:pos+ss.BytesOfProtectedData], decryptBlockLen, skipBlockLen, tmpl)
		if err != nil {
			return err
		}
		pos += ss.BytesOfProtectedData
	}

	return nil
}

func cbcsDecryptSamples(samples []mp4.FullSample, tmpl *temari.Template,
	tenc *mp4.TencBox, senc *mp4.SencBox) error {

	for i := range samples {
		var subSamplePatterns []mp4.SubSamplePattern
		if len(senc.SubSamples) != 0 {
			subSamplePatterns = senc.SubSamples[i]
		}
		err := cbcsDecryptSample(samples[i].Data, subSamplePatterns, tenc, tmpl)
		if err != nil {
			return err
		}
	}
	return nil
}

func DecryptFragment(frag *mp4.Fragment, tracks map[uint32]mp4.DecryptTrackInfo, tmpl *temari.Template) error {
	moof := frag.Moof
	var bytesRemoved uint64 = 0

	for _, traf := range moof.Trafs {
		ti, ok := tracks[traf.Tfhd.TrackID]
		if !ok {
			return fmt.Errorf("could not find decryption info for track %d", traf.Tfhd.TrackID)
		}
		if ti.Sinf == nil {
			continue
		}

		schemeType := ti.Sinf.Schm.SchemeType
		if schemeType != "cbcs" {
			return fmt.Errorf("scheme type %s not supported", schemeType)
		}
		hasSenc, isParsed := traf.ContainsSencBox()
		if !hasSenc {
			return fmt.Errorf("no senc box in traf")
		}

		var senc *mp4.SencBox
		if traf.Senc != nil {
			senc = traf.Senc
		} else {
			senc = traf.UUIDSenc.Senc
		}

		if !isParsed {
			err := senc.ParseReadBox(ti.Sinf.Schi.Tenc.DefaultPerSampleIVSize, traf.Saiz)
			if err != nil {
				return err
			}
		}

		samples, err := frag.GetFullSamples(ti.Trex)
		if err != nil {
			return err
		}

		err = cbcsDecryptSamples(samples, tmpl, ti.Sinf.Schi.Tenc, senc)
		if err != nil {
			return err
		}

		bytesRemoved += traf.RemoveEncryptionBoxes()
	}
	_, psshBytesRemoved := moof.RemovePsshs()
	bytesRemoved += psshBytesRemoved
	for _, traf := range moof.Trafs {
		for _, trun := range traf.Truns {
			trun.DataOffset -= int32(bytesRemoved)
		}
	}

	return nil
}
