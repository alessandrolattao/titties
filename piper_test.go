package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWriteTokensSkipsMultiRuneSymbols(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.txt")
	idMap := map[string]json.RawMessage{
		"_":  json.RawMessage(`[0]`),
		"a":  json.RawMessage(`[14]`),
		"aɪ": json.RawMessage(`[161]`),
		"\n": json.RawMessage(`[2]`),
		" ":  json.RawMessage(`3`),
	}
	if err := writeTokens(path, idMap); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	slices.Sort(lines)
	want := []string{"  3", "_ 0", "a 14"}
	if !slices.Equal(lines, want) {
		t.Errorf("tokens = %q, want %q", lines, want)
	}
}

func TestAppendONNXMetadataEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.onnx")
	if err := os.WriteFile(path, []byte{0x08, 0x07}, 0o600); err != nil { // ir_version = 7
		t.Fatal(err)
	}
	if err := appendONNXMetadata(path, [][2]string{{"k", "vv"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// field 14 (0x72), len 7: field 1 (0x0a) len 1 "k", field 2 (0x12) len 2 "vv"
	want := []byte{0x08, 0x07, 0x72, 0x07, 0x0a, 0x01, 'k', 0x12, 0x02, 'v', 'v'}
	if !slices.Equal(got, want) {
		t.Errorf("model = % x, want % x", got, want)
	}
}

func TestAppendVarintMultiByte(t *testing.T) {
	if got := appendVarint(nil, 300); !slices.Equal(got, []byte{0xac, 0x02}) {
		t.Errorf("varint(300) = % x", got)
	}
}

func TestPickVoicePrefersMediumThenAlphabetical(t *testing.T) {
	ids := []voiceID{
		{"it_IT", "dii-high"}, {"it_IT", "serena-medium"}, {"it_IT", "paola-medium"},
		{"it_IT", "riccardo-x_low"}, {"en_US", "amy-medium"},
	}
	got, ok := pickVoice("it_IT", ids)
	if !ok || got.name != "paola-medium" {
		t.Errorf("pickVoice = %v %v, want paola-medium", got, ok)
	}
	if _, ok := pickVoice("xx_XX", ids); ok {
		t.Error("pickVoice found a voice for an unknown language")
	}
}
