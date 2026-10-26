package fairplayrip

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// streamFixture 造一段可预测的字节流，任何位置被截断都能查出来。
func streamFixture(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i*7 + i/251)
	}
	return out
}

// byteRangeStart 解析 "bytes=N-"，没有 Range 头时返回 0。
func byteRangeStart(t *testing.T, r *http.Request) int64 {
	t.Helper()
	hdr := r.Header.Get("Range")
	if hdr == "" {
		return 0
	}
	if !strings.HasPrefix(hdr, "bytes=") || !strings.HasSuffix(hdr, "-") {
		t.Fatalf("Range 头格式意外: %q", hdr)
	}
	n, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(hdr, "bytes="), "-"), 10, 64)
	if err != nil {
		t.Fatalf("解析 Range %q: %v", hdr, err)
	}
	return n
}

// TestResumeReaderReopensWithRange：服务端第一次只在 body 里写一小段就把连接
// 掐了，客户端必须用 Range 重新打开、接着读，最终拿到的字节与原流完全一致。
func TestResumeReaderReopensWithRange(t *testing.T) {
	payload := streamFixture(64 * 1024)
	const firstChunk = 30000

	var (
		mu       sync.Mutex
		ranges   []string
		statuses []int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := byteRangeStart(t, r)
		mu.Lock()
		ranges = append(ranges, r.Header.Get("Range"))
		mu.Unlock()

		rest := payload[start:]
		if start == 0 {
			// 首次响应：声明完整长度，但只写一小段然后断开。
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
			mu.Lock()
			statuses = append(statuses, http.StatusOK)
			mu.Unlock()
			w.Write(rest[:firstChunk])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		// 续传：按 Range 把剩下的都给出去。
		w.Header().Set("Content-Length", strconv.Itoa(len(rest)))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		mu.Lock()
		statuses = append(statuses, http.StatusPartialContent)
		mu.Unlock()
		w.Write(rest)
	}))
	defer srv.Close()

	stream, err := newResumeReader(srv.URL, nil)
	if err != nil {
		t.Fatalf("newResumeReader: %v", err)
	}
	defer stream.Close()

	if got := stream.Total(); got != int64(len(payload)) {
		t.Fatalf("Total() = %d, 期望 %d", got, len(payload))
	}
	got, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("读到的字节和原流不一致: 拿到 %d 字节，期望 %d", len(got), len(payload))
	}
	if off := stream.Offset(); off != int64(len(payload)) {
		t.Fatalf("Offset() = %d, 期望 %d", off, len(payload))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(ranges) < 2 {
		t.Fatalf("只发了 %d 个请求，没有续传: %v", len(ranges), ranges)
	}
	want := fmt.Sprintf("bytes=%d-", firstChunk)
	if ranges[1] != want {
		t.Fatalf("续传请求的 Range = %q, 期望 %q", ranges[1], want)
	}
	if statuses[0] != http.StatusOK || statuses[1] != http.StatusPartialContent {
		t.Fatalf("状态码序列意外: %v", statuses)
	}
}

// TestResumeReaderServerIgnoresRange：服务端完全不认 Range，每次都整个文件重
// 发；客户端要自己跳过已经拿到的部分，最终字节依然一致。
func TestResumeReaderServerIgnoresRange(t *testing.T) {
	payload := streamFixture(64 * 1024)
	const firstChunk = 30000

	var mu sync.Mutex
	var ranges []string
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ranges = append(ranges, r.Header.Get("Range"))
		requests++
		first := requests == 1
		mu.Unlock()

		// 无视 Range：总是 200 + 整个文件，而且总是声明完整长度。
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		if first {
			// 第一次只写一小段就断，触发续传。
			w.Write(payload[:firstChunk])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		w.Write(payload)
	}))
	defer srv.Close()

	stream, err := newResumeReader(srv.URL, nil)
	if err != nil {
		t.Fatalf("newResumeReader: %v", err)
	}
	defer stream.Close()

	got, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("读到的字节和原流不一致: 拿到 %d 字节，期望 %d", len(got), len(payload))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(ranges) < 2 {
		t.Fatalf("只发了 %d 个请求，没有续传", len(ranges))
	}
	if want := fmt.Sprintf("bytes=%d-", firstChunk); ranges[1] != want {
		t.Fatalf("续传请求仍然应该带 Range %q, 实际 %q", want, ranges[1])
	}
}

// TestResumeReaderExhaustsRetryBudget：服务端每次都只给一点点就断，重试预算
// 用尽后必须报错，并带上中断时的偏移。
func TestResumeReaderExhaustsRetryBudget(t *testing.T) {
	payload := streamFixture(100 * 1024)
	const chunk = 1000

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := byteRangeStart(t, r)
		rest := payload[start:]
		n := chunk
		if n > len(rest) {
			n = len(rest)
		}
		if start == 0 {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(rest)))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		}
		w.Write(rest[:n])
		w.(http.Flusher).Flush()
		if n < len(rest) {
			panic(http.ErrAbortHandler)
		}
	}))
	defer srv.Close()

	stream, err := newResumeReader(srv.URL, nil)
	if err != nil {
		t.Fatalf("newResumeReader: %v", err)
	}
	defer stream.Close()

	got, err := io.ReadAll(stream)
	if err == nil {
		t.Fatalf("重试预算用尽时必须报错，实际读完 %d 字节", len(got))
	}
	// 首次连接 1000 字节 + 5 次续传各 1000 字节。
	wantOffset := int64(chunk) * (maxStreamRetries + 1)
	if int64(len(got)) != wantOffset {
		t.Errorf("中断前读到 %d 字节, 期望 %d", len(got), wantOffset)
	}
	msg := err.Error()
	if !strings.Contains(msg, fmt.Sprintf("offset %d", wantOffset)) {
		t.Errorf("错误信息 %q 没带上偏移 offset %d", msg, wantOffset)
	}
	if !strings.Contains(msg, fmt.Sprintf("%d reconnect attempt(s) exhausted", maxStreamRetries)) {
		t.Errorf("错误信息 %q 没说清重试次数", msg)
	}
}

// TestResumeReaderPropagatesRangeRejection：服务端对续传请求回 4xx 时不能死循
// 环，要把状态码和偏移报出来。
func TestResumeReaderPropagatesRangeRejection(t *testing.T) {
	payload := streamFixture(8 * 1024)
	const firstChunk = 2048

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload[:firstChunk])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	stream, err := newResumeReader(srv.URL, nil)
	if err != nil {
		t.Fatalf("newResumeReader: %v", err)
	}
	defer stream.Close()

	_, err = io.ReadAll(stream)
	if err == nil {
		t.Fatal("服务端拒绝 Range 时必须报错")
	}
	if !strings.Contains(err.Error(), "416") {
		t.Errorf("错误信息 %q 没带上状态码", err.Error())
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("offset %d", firstChunk)) {
		t.Errorf("错误信息 %q 没带上偏移", err.Error())
	}
}

// TestResumeReaderCleanStream 确认顺顺利利时只发一次请求、不做无谓续传。
func TestResumeReaderCleanStream(t *testing.T) {
	payload := streamFixture(4096)
	var mu sync.Mutex
	requests := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer srv.Close()

	stream, err := newResumeReader(srv.URL, nil)
	if err != nil {
		t.Fatalf("newResumeReader: %v", err)
	}
	defer stream.Close()

	got, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("读到的字节和原流不一致")
	}
	// 读完再读还是 EOF，不能又去重连。
	if _, err := stream.Read(make([]byte, 16)); err != io.EOF {
		t.Fatalf("读完之后的 Read 返回 %v, 期望 io.EOF", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != 1 {
		t.Fatalf("发了 %d 个请求, 期望 1 个", requests)
	}
}
