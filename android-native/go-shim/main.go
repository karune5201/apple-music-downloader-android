// ffmpegshim —— 内置 ffmpeg 的兼容层。
//
// 上游 amdl 在转码时给 flac/mp3 加了这组参数：
//
//	-map 0 -c:v copy -disposition:v attached_pic
//
// 其中 `-disposition` 的符号名（attached_pic）是 ffmpeg 4.3 才支持的；
// App 内置的是 ffmpeg 3.3.2（目前唯一能找到的 Android arm64 静态构建），
// 只会把 attached_pic 当作未定义常量，进程直接退出 1 —— 也就是用户看到的
// “Conversion failed: exit status 1”。
//
// 而且 3.3.2 无法给输出流设置 attached_pic，也就无法用 ffmpeg 自己把封面写进
// flac/mp3。所以这层 shim 做三件事：
//
//  1. 把上面的封面参数改写为 `-vn`（只转音频），让转码先能成功；
//  2. 转完后由本程序把封面写回输出文件：
//     flac 用 METADATA_BLOCK_PICTURE，mp3 用 ID3v2 APIC，均为纯 Go 实现；
//  3. 把真实 ffmpeg 的退出码原样返回。
//
// 这样无论内置哪个版本的 ffmpeg，转码行为都一致，也不依赖上游代码的细节。
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 真实 ffmpeg：与本程序同目录的 libffmpeg.so
func realFFmpeg() string {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), "libffmpeg.so")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "ffmpeg"
}

// 需要被丢掉的“封面三件套”，以及 mp3 的流标签参数
func isCoverOpt(arg string) bool {
	return arg == "-disposition" || strings.HasPrefix(arg, "-disposition:") ||
		strings.HasPrefix(arg, "-metadata:s:v")
}

// 重写上游参数：去掉封面相关项，改成 -vn。
// 返回新参数、是否改写过、输入文件、输出文件。
func rewrite(args []string) (out []string, stripped bool, in, dst string) {
	out = make([]string, 0, len(args)+2)
	skipNext := false
	for i := 0; i < len(args); i++ {
		if skipNext {
			skipNext = false
			continue
		}
		arg := args[i]
		if isCoverOpt(arg) {
			stripped = true
			skipNext = true // 连同它的值一起丢掉
			continue
		}
		if stripped || willStrip(args, i) {
			// -map 0 与 -c:v copy 只在封面场景出现，一并去掉
			if arg == "-map" && i+1 < len(args) && args[i+1] == "0" {
				stripped = true
				skipNext = true
				continue
			}
			if arg == "-c:v" && i+1 < len(args) && args[i+1] == "copy" {
				stripped = true
				skipNext = true
				continue
			}
		}
		if arg == "-i" && i+1 < len(args) {
			in = args[i+1]
		}
		out = append(out, arg)
	}
	if n := len(out); n > 0 {
		dst = out[n-1]
	}
	if stripped {
		// -vn 必须出现在输出文件之前
		if len(out) > 0 {
			out = append(out[:len(out)-1], "-vn", out[len(out)-1])
		} else {
			out = append(out, "-vn")
		}
	}
	return out, stripped, in, dst
}

// 提前判断这组参数里是否有封面选项（-map 0 要等确认后再删）
func willStrip(args []string, _ int) bool {
	for _, a := range args {
		if isCoverOpt(a) {
			return true
		}
	}
	return false
}

// 用真实 ffmpeg 把源文件的封面导出来（attached_pic 是普通视频流）。
func extractCover(ffmpeg, src string) ([]byte, error) {
	cmd := exec.Command(ffmpeg, "-v", "error", "-i", src,
		"-map", "0:v:0", "-c", "copy", "-f", "image2pipe", "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("提取封面失败: %v %s", err, stderr.String())
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("源文件没有封面")
	}
	return stdout.Bytes(), nil
}

func mimeOf(data []byte) string {
	if len(data) > 8 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return "image/png"
	}
	return "image/jpeg"
}

// ---- FLAC：插入 METADATA_BLOCK_PICTURE ----

func injectFlacCover(path string, cover []byte) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 4 || string(data[:4]) != "fLaC" {
		return fmt.Errorf("不是 flac 文件")
	}

	// 收集元数据块，清掉旧封面块
	type block struct {
		typ  byte
		body []byte
	}
	var blocks []block
	pos := 4
	for pos+4 <= len(data) {
		h := data[pos]
		typ := h & 0x7f
		last := h&0x80 != 0
		length := int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		start := pos + 4
		if start+length > len(data) {
			return fmt.Errorf("元数据块长度越界")
		}
		if typ != 6 { // 丢掉已有的图片块，避免重复
			blocks = append(blocks, block{typ, data[start : start+length]})
		}
		pos = start + length
		if last {
			break
		}
	}
	audio := data[pos:]

	var buf bytes.Buffer
	buf.WriteString("fLaC")
	for _, b := range blocks {
		// 所有原有块都清掉 last 位：新的最后一块是下面的图片块
		writeFlacHeader(&buf, "", b.typ, len(b.body))
		buf.Write(b.body)
	}
	body := buildFlacPicture(cover)
	writeFlacHeader(&buf, "", 0x86, len(body)) // type 6 = PICTURE，带 last 位
	buf.Write(body)
	buf.Write(audio)

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func writeFlacHeader(buf *bytes.Buffer, _ string, header byte, length int) {
	buf.WriteByte(header)
	buf.WriteByte(byte(length >> 16))
	buf.WriteByte(byte(length >> 8))
	buf.WriteByte(byte(length))
}

func buildFlacPicture(cover []byte) []byte {
	var b bytes.Buffer
	w32 := func(v uint32) {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], v)
		b.Write(tmp[:])
	}
	mime := mimeOf(cover)
	w32(3) // 图片类型：3 = 封面（正面）
	w32(uint32(len(mime)))
	b.WriteString(mime)
	w32(0)             // 描述
	w32(0)             // 宽（0 = 未知，播放器会自己解析）
	w32(0)             // 高
	w32(0)             // 色深
	w32(0)             // 索引色数
	w32(uint32(len(cover)))
	b.Write(cover)
	return b.Bytes()
}

// ---- MP3：插入 ID3v2 APIC ----

func syncsafe(n int) []byte {
	return []byte{byte((n >> 21) & 0x7f), byte((n >> 14) & 0x7f), byte((n >> 7) & 0x7f), byte(n & 0x7f)}
}

func fromSyncsafe(b []byte) int {
	return int(b[0])<<21 | int(b[1])<<14 | int(b[2])<<7 | int(b[3])
}

// 图片帧：ID3v2.3/2.4 用 APIC（帧 ID 4 字节，载荷带 MIME）；
// 只有 ID3v2.2 才用 PIC（3 字符图片格式）。写错帧名读取方会直接忽略封面。
func buildCoverFrame(cover []byte, version byte) []byte {
	var body bytes.Buffer
	body.WriteByte(0) // 文本编码：ISO-8859-1
	if version >= 3 {
		body.WriteString(mimeOf(cover))
		body.WriteByte(0) // MIME 结尾
	} else {
		if mimeOf(cover) == "image/png" {
			body.WriteString("PNG")
		} else {
			body.WriteString("JPG")
		}
	}
	body.WriteByte(3) // 图片类型：封面（正面）
	body.WriteByte(0) // 描述结尾
	body.Write(cover)

	var frame bytes.Buffer
	if version == 2 {
		frame.WriteString("PIC")
	} else {
		frame.WriteString("APIC")
	}
	if version >= 4 {
		frame.Write(syncsafe(body.Len()))
	} else {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], uint32(body.Len()))
		frame.Write(tmp[:])
	}
	frame.Write([]byte{0, 0}) // 标志位
	frame.Write(body.Bytes())
	return frame.Bytes()
}

// 返回标签体里“帧区”的结束偏移，其后是填充。
func framesEnd(body []byte, version byte) int {
	p := 0
	for p+10 <= len(body) {
		if body[p] == 0 { // 帧 ID 为 0 表示进入填充
			break
		}
		var size int
		if version >= 4 {
			size = fromSyncsafe(body[p+4 : p+8])
		} else {
			size = int(binary.BigEndian.Uint32(body[p+4 : p+8]))
		}
		if size < 0 || p+10+size > len(body) {
			break
		}
		p += 10 + size
	}
	return p
}

func injectMp3Cover(path string, cover []byte) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dst := path + ".tmp"

	if len(data) >= 10 && string(data[:3]) == "ID3" {
		version := data[3]
		size := fromSyncsafe(data[6:10])
		tagEnd := 10 + size
		if tagEnd > len(data) {
			return fmt.Errorf("ID3 长度越界")
		}
		body := data[10:tagEnd]
		// 按帧结构算出帧区结束位置：不能靠“裁掉尾部零字节”，
		// 因为最后一帧的内容本身可能以 0x00 结尾（ffmpeg 的 TSSE 就是这样）。
		end := framesEnd(body, version)
		frame := buildCoverFrame(cover, version)
		newBody := make([]byte, 0, len(body)+len(frame))
		newBody = append(newBody, body[:end]...)
		newBody = append(newBody, frame...)
		newBody = append(newBody, body[end:]...) // 保留原有填充
		out := make([]byte, 0, 10+len(newBody)+len(data)-tagEnd)
		out = append(out, data[:3]...)
		out = append(out, version, data[4], data[5])
		out = append(out, syncsafe(len(newBody))...)
		out = append(out, newBody...)
		out = append(out, data[tagEnd:]...)
		if err := os.WriteFile(dst, out, 0644); err != nil {
			return err
		}
	} else {
		// 没有标签：新建一个 ID3v2.4
		frame := buildCoverFrame(cover, 4)
		out := make([]byte, 0, 10+len(frame)+len(data))
		out = append(out, 'I', 'D', '3', 4, 0, 0)
		out = append(out, syncsafe(len(frame))...)
		out = append(out, frame...)
		out = append(out, data...)
		if err := os.WriteFile(dst, out, 0644); err != nil {
			return err
		}
	}
	return os.Rename(dst, path)
}

func main() {
	real := realFFmpeg()
	args, stripped, src, dst := rewrite(os.Args[1:])

	cmd := exec.Command(real, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		code := 1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		os.Exit(code)
	}

	// 转码成功：把封面补回去（opus/wav 本身不支持封面，跳过）
	if !stripped || src == "" || dst == "" {
		return
	}
	ext := strings.ToLower(filepath.Ext(dst))
	if ext != ".flac" && ext != ".mp3" {
		return
	}
	cover, err := extractCover(real, src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ffmpegshim: 跳过封面:", err)
		return
	}
	if ext == ".flac" {
		err = injectFlacCover(dst, cover)
	} else {
		err = injectMp3Cover(dst, cover)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ffmpegshim: 写入封面失败:", err)
	}
}
