package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func blocksOf(t *testing.T, path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:4]) != "fLaC" {
		t.Fatal("不是 flac")
	}
	pos := 4
	var types []byte
	for pos+4 <= len(data) {
		h := data[pos]
		typ := h & 0x7f
		last := h&0x80 != 0
		length := int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		types = append(types, typ)
		t.Logf("block type=%d len=%d last=%v", typ, length, last)
		pos += 4 + length
		if last {
			break
		}
	}
	return types
}

func TestInjectFlac(t *testing.T) {
	ff := "/tmp/conv/ffmpeg"
	out := "/tmp/conv/unit.flac"
	os.Remove(out)
	// 只转音频
	cmd := exec.Command(ff, "-y", "-i", "/tmp/conv/src.m4a", "-vn", "-c:a", "flac", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成失败: %v %s", err, b)
	}
	cover, err := os.ReadFile("/tmp/conv/cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := injectFlacCover(out, cover); err != nil {
		t.Fatalf("注入失败: %v", err)
	}
	types := blocksOf(t, out)
	hasPicture := bytes.Contains(types, []byte{6})
	t.Logf("块类型序列: %v", types)
	if !hasPicture {
		t.Fatalf("没有 PICTURE 块")
	}
	// 真能读出来吗
	probe := exec.Command(ff, "-v", "error", "-i", out, "-map", "0:v:0", "-c", "copy", "-f", "image2pipe", "-")
	var buf bytes.Buffer
	probe.Stdout = &buf
	if err := probe.Run(); err != nil {
		t.Fatalf("读取封面失败: %v", err)
	}
	t.Logf("从 flac 读出的封面字节数: %d", buf.Len())
	if buf.Len() < 1000 {
		t.Fatalf("封面数据异常")
	}
	_ = filepath.Base
}

func TestInjectMp3(t *testing.T) {
	ff := "/tmp/conv/ffmpeg"
	out := "/tmp/conv/unit.mp3"
	os.Remove(out)
	cmd := exec.Command(ff, "-y", "-i", "/tmp/conv/src.m4a", "-vn", "-c:a", "libmp3lame",
		"-qscale:a", "2", "-id3v2_version", "3", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("生成失败: %v %s", err, b)
	}
	cover, err := os.ReadFile("/tmp/conv/cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := injectMp3Cover(out, cover); err != nil {
		t.Fatalf("注入失败: %v", err)
	}
	// 用 ffmpeg 读回封面
	probe := exec.Command(ff, "-v", "error", "-i", out, "-map", "0:v:0", "-c", "copy", "-f", "image2pipe", "-")
	var buf bytes.Buffer
	probe.Stdout = &buf
	if err := probe.Run(); err != nil {
		t.Fatalf("读取封面失败: %v", err)
	}
	t.Logf("从 mp3 读出的封面字节数: %d", buf.Len())
	if buf.Len() < 1000 {
		t.Fatalf("封面数据异常: %d", buf.Len())
	}
	// 音轨仍然可用
	check := exec.Command(ff, "-v", "error", "-i", out, "-f", "null", "-")
	if err := check.Run(); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
}
