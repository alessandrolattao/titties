package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Voices missing from the sherpa-onnx release come straight from the official Piper catalog
// and are converted the way sherpa-onnx's scripts/piper/add_meta_data.py does it.
const (
	piperCatalogURL = "https://huggingface.co/rhasspy/piper-voices/resolve/main/voices.json"
	piperFilesURL   = "https://huggingface.co/rhasspy/piper-voices/resolve/main/"
	espeakDataURL   = releaseURL + "espeak-ng-data.tar.bz2"
)

type piperEntry struct {
	Files map[string]json.RawMessage `json:"files"`
}

// piperCatalog lists the official Piper voices, keyed by id (it_IT-serena-high).
func piperCatalog() (map[string]piperEntry, error) {
	var voices map[string]piperEntry
	if err := getJSON(piperCatalogURL, &voices); err != nil {
		return nil, err
	}
	return voices, nil
}

// piperVoiceID turns it_IT-serena-high into {it_IT, serena-high}.
func piperVoiceID(key string) (voiceID, bool) {
	lang, name, ok := strings.Cut(key, "-")
	return voiceID{lang: lang, name: name}, ok
}

// installPiperVoice downloads an official Piper voice into dir and makes it loadable by sherpa-onnx.
func installPiperVoice(id voiceID, dir string) error {
	voices, err := piperCatalog()
	if err != nil {
		return err
	}
	key := id.lang + "-" + id.name
	entry, ok := voices[key]
	if !ok {
		return fmt.Errorf("voice %s %s: %w", id.lang, id.name, errNotFound)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	paths := make([]string, 0, len(entry.Files))
	for p := range entry.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := fetchFile(piperFilesURL+escapePath(p), filepath.Join(dir, filepath.Base(p))); err != nil {
			return err
		}
	}

	model := filepath.Join(dir, key+".onnx")
	conf, err := readPiperConfig(model + ".json")
	if err != nil {
		return err
	}
	if err := writeTokens(filepath.Join(dir, "tokens.txt"), conf.PhonemeIDMap); err != nil {
		return err
	}
	return appendONNXMetadata(model, conf.sherpaMetadata())
}

type piperConfig struct {
	Audio struct {
		SampleRate int `json:"sample_rate"`
	} `json:"audio"`
	Espeak struct {
		Voice string `json:"voice"`
	} `json:"espeak"`
	LangCode    string `json:"lang_code"`
	PhonemeType string `json:"phoneme_type"`
	NumSpeakers int    `json:"num_speakers"`
	Language    struct {
		NameEnglish string `json:"name_english"`
	} `json:"language"`
	PhonemeIDMap map[string]json.RawMessage `json:"phoneme_id_map"`
}

func readPiperConfig(path string) (piperConfig, error) {
	var c piperConfig
	data, err := os.ReadFile(path) //nolint:gosec // our own data dir
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

// sherpaMetadata is what sherpa-onnx expects in a Piper model, as add_meta_data.py writes it.
func (c piperConfig) sherpaMetadata() [][2]string {
	rate := c.Audio.SampleRate
	if rate == 22500 {
		rate = 22050
	}
	voice := c.Espeak.Voice
	if c.LangCode != "" {
		voice = c.LangCode
	}
	if voice == "pt-PT" {
		voice = "pt"
	}
	hasEspeak, hasG2pw := "1", "0"
	if c.PhonemeType == "pinyin" && voice == "zh" {
		hasEspeak, hasG2pw = "0", "1"
	}
	return [][2]string{
		{"model_type", "vits"},
		{"comment", "piper"},
		{"language", c.Language.NameEnglish},
		{"voice", voice},
		{"version", "1"},
		{"has_espeak", hasEspeak},
		{"has_g2pw", hasG2pw},
		{"n_speakers", strconv.Itoa(c.NumSpeakers)},
		{"sample_rate", strconv.Itoa(rate)},
	}
}

// writeTokens writes sherpa-onnx's tokens.txt from Piper's phoneme_id_map: "symbol id" per line.
func writeTokens(path string, idMap map[string]json.RawMessage) error {
	var b strings.Builder
	for sym, raw := range idMap {
		// sherpa-onnx reads one codepoint per token, and the phonemizer emits single codepoints anyway.
		if sym == "\n" || utf8.RuneCountInString(sym) != 1 {
			continue
		}
		var ids []int
		if err := json.Unmarshal(raw, &ids); err != nil || len(ids) == 0 {
			var id int
			if err := json.Unmarshal(raw, &id); err != nil {
				return fmt.Errorf("phoneme_id_map %q: %w", sym, err)
			}
			ids = []int{id}
		}
		fmt.Fprintf(&b, "%s %d\n", sym, ids[0])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // not secret
}

// appendONNXMetadata adds metadata_props to an ONNX model without rewriting it: protobuf merges
// repeated fields of concatenated messages, so encoded entries appended at the end are read as
// part of the ModelProto (field 14, StringStringEntryProto{key: 1, value: 2}).
func appendONNXMetadata(model string, props [][2]string) error {
	var buf []byte
	for _, kv := range props {
		var entry []byte
		entry = appendBytesField(entry, 1, []byte(kv[0]))
		entry = appendBytesField(entry, 2, []byte(kv[1]))
		buf = appendBytesField(buf, 14, entry)
	}
	f, err := os.OpenFile(model, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec // our own data dir
	if err != nil {
		return err
	}
	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func appendBytesField(b []byte, field int, value []byte) []byte {
	b = appendVarint(b, uint64(field)<<3|2) //nolint:gosec // small field numbers
	b = appendVarint(b, uint64(len(value)))
	return append(b, value...)
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// ensureEspeakData downloads the espeak-ng-data the phonemizer needs for voices that do not ship it.
func ensureEspeakData(root string) (string, error) {
	dir := filepath.Join(root, "espeak-ng-data")
	if _, err := os.Stat(filepath.Join(dir, "phontab")); err == nil {
		return dir, nil
	}
	tmp, err := os.MkdirTemp(root, ".download-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	fmt.Fprintln(os.Stderr, "downloading espeak-ng data...")
	if err := download(espeakDataURL, tmp); err != nil {
		return "", fmt.Errorf("espeak-ng data: %w", err)
	}
	if err := os.Rename(filepath.Join(tmp, "espeak-ng-data"), dir); err != nil {
		return "", err
	}
	return dir, nil
}

func fetchFile(rawURL, path string) error {
	resp, err := http.Get(rawURL) //nolint:gosec,noctx // fixed Hugging Face URL, one-shot CLI
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return writeFile(path, resp.Body)
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// errNotFound marks a voice package that does not exist, so ensureVoice can try the Piper catalog.
var errNotFound = errors.New("not found")
