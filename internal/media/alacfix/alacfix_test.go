package alacfix

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amdl/internal/i18n"
	"amdl/internal/logx"
)

func makeAtom(typ string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[0:4], uint32(len(b)))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

func makeTkhd() []byte {
	payload := make([]byte, 24)
	payload[0] = 0                                // version
	binary.BigEndian.PutUint32(payload[12:16], 1) // trackID = 1
	return makeAtom("tkhd", payload)
}

func makeHdlr() []byte {
	payload := make([]byte, 24)
	copy(payload[8:12], "soun")
	return makeAtom("hdlr", payload)
}

func makeAlacMagicCookie() []byte {
	cookie := make([]byte, 28)
	binary.BigEndian.PutUint32(cookie[4:8], 4096) // maxSamplesPerFrame
	cookie[9] = 16                                // sampleSize
	cookie[10] = 40                               // riceHistoryMult
	cookie[11] = 10                               // riceInitialHistory
	cookie[12] = 14                               // riceLimit
	cookie[13] = 2                                // channels
	return makeAtom("alac", cookie)
}

func makeStsd() []byte {
	alacBox := makeAlacMagicCookie()
	audioHdr := make([]byte, 28)
	sampleEntryPayload := append(audioHdr, alacBox...)
	sampleEntry := makeAtom("alac", sampleEntryPayload)

	stsdBody := make([]byte, 8)
	binary.BigEndian.PutUint32(stsdBody[4:8], 1) // entryCount = 1
	stsdBody = append(stsdBody, sampleEntry...)
	return makeAtom("stsd", stsdBody)
}

func makeStbl(packetOffset int, packetSizes []int) []byte {
	stsd := makeStsd()

	// stsz
	stszBody := make([]byte, 12+4*len(packetSizes))
	binary.BigEndian.PutUint32(stszBody[8:12], uint32(len(packetSizes))) // count
	for i, sz := range packetSizes {
		binary.BigEndian.PutUint32(stszBody[12+4*i:16+4*i], uint32(sz))
	}
	stsz := makeAtom("stsz", stszBody)

	// stsc
	stscBody := make([]byte, 20)
	binary.BigEndian.PutUint32(stscBody[4:8], 1)                          // count = 1
	binary.BigEndian.PutUint32(stscBody[8:12], 1)                         // firstChunk = 1
	binary.BigEndian.PutUint32(stscBody[12:16], uint32(len(packetSizes))) // samplesPerChunk
	binary.BigEndian.PutUint32(stscBody[16:20], 1)                        // sampleDescIndex = 1
	stsc := makeAtom("stsc", stscBody)

	// stco
	stcoBody := make([]byte, 12)
	binary.BigEndian.PutUint32(stcoBody[4:8], 1)                     // count = 1
	binary.BigEndian.PutUint32(stcoBody[8:12], uint32(packetOffset)) // chunkOff = packetOffset
	stco := makeAtom("stco", stcoBody)

	var stblPayload []byte
	stblPayload = append(stblPayload, stsd...)
	stblPayload = append(stblPayload, stsz...)
	stblPayload = append(stblPayload, stsc...)
	stblPayload = append(stblPayload, stco...)
	return makeAtom("stbl", stblPayload)
}

func makeMoov(packetOffset int, packetSizes []int) []byte {
	tkhd := makeTkhd()
	hdlr := makeHdlr()
	stbl := makeStbl(packetOffset, packetSizes)
	minf := makeAtom("minf", stbl)
	mdia := makeAtom("mdia", append(hdlr, minf...))
	trak := makeAtom("trak", append(tkhd, mdia...))
	return makeAtom("moov", trak)
}

// createTestPacket builds an ALAC uncompressed element packet.
// Total element body: 87 bits.
// If withEndTag is true, bits 87..89 are 111 (TYPE_END).
// If false, bits 87..89 are 000.
func createTestPacket(withEndTag bool) []byte {
	buf := make([]byte, 16) // 128 bits
	// elem = 1 (bits 0..2: 001)
	buf[0] = 0x20
	// bits 3..6: 0000 (skip 4)
	// bits 7..18: 000000000000 (skip 12)
	// bit 19: hasSize = 1 -> set bit 3 of buf[2]
	buf[2] |= (1 << 4)
	// bits 20..21: extraBits = 00
	// bit 22: notCompressed = 1 -> set bit 1 of buf[2]
	buf[2] |= (1 << 1)
	// bits 23..54: outputSamples = 1 (32 bits)
	// Setting outputSamples = 1: bit 54 is bit 1 of buf[6]
	buf[6] |= (1 << 1)
	// bits 55..86 (32 bits): uncompressed samples = 0
	// bit 87..89: tag
	if withEndTag {
		// byte 10 is bits 80..87. bit 87 is bit 0 of buf[10].
		buf[10] |= (1 << 0)
		// byte 11 is bits 88..95. bits 88 and 89 are bits 7 and 6 of buf[11].
		buf[11] |= (3 << 6)
	}
	return buf
}

func buildTestMP4Multi(t *testing.T, withEndTags []bool) string {
	t.Helper()
	path, _ := buildTestMP4MultiAt(t, withEndTags)
	return path
}

// buildTestMP4MultiAt also reports the file offset of the first packet, which
// the verbose assertions need because the report prints raw offsets.
func buildTestMP4MultiAt(t *testing.T, withEndTags []bool) (string, int64) {
	t.Helper()
	var pktPayload []byte
	var sizes []int
	for _, endTag := range withEndTags {
		pkt := createTestPacket(endTag)
		pktPayload = append(pktPayload, pkt...)
		sizes = append(sizes, len(pkt))
	}
	placeholderMoov := makeMoov(0, sizes)
	moovSize := len(placeholderMoov)
	mdatHdrSize := 8
	packetOffset := moovSize + mdatHdrSize

	moov := makeMoov(packetOffset, sizes)
	mdat := makeAtom("mdat", pktPayload)

	var full []byte
	full = append(full, moov...)
	full = append(full, mdat...)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.m4a")
	if err := os.WriteFile(path, full, 0644); err != nil {
		t.Fatal(err)
	}
	return path, int64(packetOffset)
}

func buildTestMP4(t *testing.T, withEndTag bool) string {
	return buildTestMP4Multi(t, []bool{withEndTag})
}

// captureLog collects everything the package writes through logx while f runs.
//
// logx keeps its own writer instead of following os.Stdout, so redirecting
// os.Stdout (as the old captureStdout helper did) would capture nothing.
func captureLog(f func() error) (string, error) {
	var buf bytes.Buffer
	logx.SetOutput(&buf)
	defer logx.SetOutput(os.Stdout)
	err := f()
	return buf.String(), err
}

// stripMarker drops the leading level marker logx adds itself, so assertions
// compare the message body only and therefore hold in every language.
func stripMarker(line string) string {
	line = strings.TrimSpace(line)
	for _, marker := range []string{
		logx.MarkerError, logx.MarkerWarn, logx.MarkerOK, logx.MarkerStep, logx.MarkerDetail,
	} {
		if rest, ok := strings.CutPrefix(line, marker); ok {
			return strings.TrimSpace(rest)
		}
	}
	return line
}

// withLanguages runs check once per supported language so the assertions prove
// the output follows i18n.T() in zh, ja and en.
func withLanguages(t *testing.T, check func(lang i18n.Lang)) {
	t.Helper()
	defer i18n.Set(string(i18n.Default))
	for _, lang := range i18n.All {
		i18n.Set(string(lang))
		check(lang)
	}
}

// hasMarker reports whether the line carries the expected level marker.
func hasMarker(line, marker string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), marker)
}

func TestAlacFix_IntactAudio(t *testing.T) {
	path := buildTestMP4(t, true)

	res, err := Fix(path, false)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if res.TracksCount != 1 {
		t.Fatalf("expected 1 track, got %d", res.TracksCount)
	}
	if res.Patched != 0 {
		t.Fatalf("expected 0 patched, got %d", res.Patched)
	}
	if res.Unrepaired != 0 {
		t.Fatalf("expected 0 unrepaired, got %d", res.Unrepaired)
	}

	withLanguages(t, func(lang i18n.Lang) {
		out, err := captureLog(func() error {
			return Run(path, false)
		})
		if err != nil {
			t.Fatalf("[%s] Run failed: %v", lang, err)
		}
		if got, want := stripMarker(out), i18n.T("media.alacfix.intact"); got != want {
			t.Fatalf("[%s] expected %q, got %q (raw %q)", lang, want, got, out)
		}
		if !hasMarker(out, logx.MarkerOK) {
			t.Fatalf("[%s] expected an OK marker, got %q", lang, out)
		}
	})
}

func TestAlacFix_RepairedAudio(t *testing.T) {
	path := buildTestMP4(t, false)

	// First verify Fix
	res, err := Fix(path, false)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if res.TracksCount != 1 {
		t.Fatalf("expected 1 track, got %d", res.TracksCount)
	}
	if res.Patched != 1 {
		t.Fatalf("expected 1 patched, got %d", res.Patched)
	}

	// Now re-create malformed file and test Run. Run repairs in place, so every
	// language needs its own copy.
	var repairedPath string
	withLanguages(t, func(lang i18n.Lang) {
		fresh := buildTestMP4(t, false)
		repairedPath = fresh
		out, err := captureLog(func() error {
			return Run(fresh, false)
		})
		if err != nil {
			t.Fatalf("[%s] Run failed: %v", lang, err)
		}
		if got, want := stripMarker(out), i18n.T("media.alacfix.repaired", 1); got != want {
			t.Fatalf("[%s] expected %q, got %q (raw %q)", lang, want, got, out)
		}
		if !hasMarker(out, logx.MarkerOK) {
			t.Fatalf("[%s] expected an OK marker, got %q", lang, out)
		}
	})

	// The file Run just repaired must now scan clean.
	out, err := captureLog(func() error {
		return Run(repairedPath, false)
	})
	if err != nil {
		t.Fatalf("Run failed on repaired file: %v", err)
	}
	if got, want := stripMarker(out), i18n.T("media.alacfix.intact"); got != want {
		t.Fatalf("expected %q, got %q (raw %q)", want, got, out)
	}
}

func TestAlacFix_NonAlacFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dummy.mp4")
	moov := makeAtom("moov", []byte("dummy payload"))
	if err := os.WriteFile(path, moov, 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Fix(path, false)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if res.TracksCount != 0 {
		t.Fatalf("expected 0 tracks, got %d", res.TracksCount)
	}

	out, err := captureLog(func() error {
		return Run(path, false)
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output for non-alac, got %q", out)
	}
}

func TestAlacFix_MultiplePackets(t *testing.T) {
	// 3 packets: 2 malformed, 1 intact
	path := buildTestMP4Multi(t, []bool{false, true, false})

	res, err := Fix(path, false)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if res.TracksCount != 1 {
		t.Fatalf("expected 1 track, got %d", res.TracksCount)
	}
	if res.Patched != 2 {
		t.Fatalf("expected 2 patched, got %d", res.Patched)
	}

	withLanguages(t, func(lang i18n.Lang) {
		fresh := buildTestMP4Multi(t, []bool{false, true, false})
		out, err := captureLog(func() error {
			return Run(fresh, false)
		})
		if err != nil {
			t.Fatalf("[%s] Run failed: %v", lang, err)
		}
		if got, want := stripMarker(out), i18n.T("media.alacfix.repaired", 2); got != want {
			t.Fatalf("[%s] expected %q, got %q (raw %q)", lang, want, got, out)
		}
	})
}

// RunVerbose must show its per-track/per-packet details even when --verbose
// was never enabled globally (that is how cmd/alacfix calls it), so those lines
// have to come from Detail/Warn rather than Debug.
func TestAlacFix_RunVerboseVisibleWithoutGlobalVerbose(t *testing.T) {
	defer logx.SetVerbose(logx.Verbose())
	logx.SetVerbose(false)

	withLanguages(t, func(lang i18n.Lang) {
		path, packetOffset := buildTestMP4MultiAt(t, []bool{false})

		out, err := captureLog(func() error {
			return RunVerbose(path, false)
		})
		if err != nil {
			t.Fatalf("[%s] RunVerbose failed: %v", lang, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("[%s] RunVerbose printed nothing without global verbose", lang)
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		bodies := make([]string, 0, len(lines))
		for _, line := range lines {
			bodies = append(bodies, stripMarker(line))
		}
		joined := strings.Join(bodies, "\n")

		wants := []string{
			i18n.T("media.alacfix.track_params", 1, 1, 4096, 16, 2),
			i18n.T("media.alacfix.packet_damaged", 1, 0, packetOffset, logx.Bytes(16), 87),
			i18n.T("media.alacfix.track_damaged", 1, 1),
			i18n.T("media.alacfix.patch_row", 1, 0, 87, 87, 128),
			i18n.T("media.alacfix.repaired", 1),
		}
		for _, want := range wants {
			if !strings.Contains(joined, want) {
				t.Fatalf("[%s] verbose output missing %q\n--- got ---\n%s", lang, want, out)
			}
		}
		if !strings.Contains(out, logx.MarkerWarn) {
			t.Fatalf("[%s] expected a warning marker in verbose output, got %q", lang, out)
		}
		if !strings.Contains(out, logx.MarkerDetail) {
			t.Fatalf("[%s] expected a detail marker in verbose output, got %q", lang, out)
		}
	})
}

func TestAlacFix_OutputPathLeavesInputUntouched(t *testing.T) {
	path := buildTestMP4Multi(t, []bool{false, true, false})
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.m4a")

	res, err := Fix(path, false, out)
	if err != nil {
		t.Fatalf("Fix failed: %v", err)
	}
	if res.Patched != 2 {
		t.Fatalf("expected 2 patched, got %d", res.Patched)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(orig, after) {
		t.Fatal("input file was modified")
	}

	res, err = Fix(out, false)
	if err != nil {
		t.Fatalf("Fix on output failed: %v", err)
	}
	if res.Patched != 0 {
		t.Fatalf("expected repaired output, got %d packets still patched", res.Patched)
	}
}
