package main

import (
	"archive/tar"
	"compress/bzip2"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	releaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/"
	releaseAPI = "https://api.github.com/repos/k2-fsa/sherpa-onnx/releases/tags/tts-models"
	pkgPrefix  = "vits-piper-"
	pkgSuffix  = ".tar.bz2"
)

// voiceID is a Piper voice: its language (it_IT) and its name (paola-medium).
type voiceID struct {
	lang string
	name string
}

func (id voiceID) pkg() string { return pkgPrefix + id.lang + "-" + id.name }

type voice struct {
	model  string
	tokens string
	espeak string // espeak-ng-data shipped in the voice package, used by the phonemizer
}

// catalogTTL is how long the saved catalog is used before asking GitHub and Hugging Face again:
// the unauthenticated GitHub API allows 60 requests an hour, and listing takes several.
const catalogTTL = 24 * time.Hour

// catalog lists every voice titties can use, from the saved copy when it is fresh enough.
// When the sources cannot be reached, a stale copy is better than nothing.
func catalog() ([]voiceID, error) {
	root, err := dataDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "catalog.json")
	cached, age, cacheErr := readCatalog(path)
	if cacheErr == nil && age < catalogTTL {
		return cached, nil
	}
	ids, err := fetchCatalog()
	if err != nil {
		if cacheErr == nil {
			fmt.Fprintf(os.Stderr, "titties: using the saved voice catalog (%v)\n", err)
			return cached, nil
		}
		return nil, err
	}
	_ = writeCatalog(path, ids)
	return ids, nil
}

type catalogEntry struct {
	Lang string `json:"lang"`
	Name string `json:"name"`
}

func readCatalog(path string) ([]voiceID, time.Duration, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // our own data dir
	if err != nil {
		return nil, 0, err
	}
	var entries []catalogEntry
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
		return nil, 0, errors.New("empty or broken catalog cache")
	}
	ids := make([]voiceID, len(entries))
	for i, e := range entries {
		ids[i] = voiceID{lang: e.Lang, name: e.Name}
	}
	return ids, time.Since(st.ModTime()), nil
}

func writeCatalog(path string, ids []voiceID) error {
	entries := make([]catalogEntry, len(ids))
	for i, id := range ids {
		entries[i] = catalogEntry{Lang: id.lang, Name: id.name}
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { //nolint:gosec // not secret
		return err
	}
	return os.Rename(tmp, path)
}

// fetchCatalog asks for the sherpa-onnx packages plus the official Piper voices sherpa-onnx
// does not repackage, which are converted on download.
func fetchCatalog() ([]voiceID, error) {
	ids, err := sherpaCatalog()
	if err != nil {
		return nil, err
	}
	official, err := piperCatalog()
	if err != nil {
		return nil, err
	}
	seen := make(map[voiceID]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	for key := range official {
		if id, ok := piperVoiceID(key); ok && !seen[id] {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b voiceID) int {
		return strings.Compare(strings.ToLower(a.lang+"-"+a.name), strings.ToLower(b.lang+"-"+b.name))
	})
	return ids, nil
}

// sherpaCatalog lists the Piper voices published in the sherpa-onnx tts-models release.
func sherpaCatalog() ([]voiceID, error) {
	var rel struct {
		ID int64 `json:"id"`
	}
	if err := getJSON(releaseAPI, &rel); err != nil {
		return nil, err
	}

	var ids []voiceID
	for page := 1; ; page++ {
		var assets []struct {
			Name string `json:"name"`
		}
		url := fmt.Sprintf("https://api.github.com/repos/k2-fsa/sherpa-onnx/releases/%d/assets?per_page=100&page=%d", rel.ID, page)
		if err := getJSON(url, &assets); err != nil {
			return nil, err
		}
		for _, a := range assets {
			if id, ok := parsePkg(a.Name); ok {
				ids = append(ids, id)
			}
		}
		if len(assets) < 100 {
			break
		}
	}
	return ids, nil
}

// parsePkg turns vits-piper-it_IT-paola-medium.tar.bz2 into {it_IT, paola-medium}; quantized variants are skipped.
func parsePkg(file string) (voiceID, bool) {
	if !strings.HasPrefix(file, pkgPrefix) || !strings.HasSuffix(file, pkgSuffix) {
		return voiceID{}, false
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(file, pkgPrefix), pkgSuffix)
	if strings.HasSuffix(rest, "-int8") || strings.HasSuffix(rest, "-fp16") {
		return voiceID{}, false
	}
	lang, name, ok := strings.Cut(rest, "-")
	if !ok {
		return voiceID{}, false
	}
	return voiceID{lang: lang, name: name}, true
}

func getJSON(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:noctx // one-shot CLI
	if err != nil {
		return err
	}
	// A token lifts the GitHub API limit from 60 to 5000 requests an hour.
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // fixed GitHub and Hugging Face URLs
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// dataDir is where voices are stored: $XDG_DATA_HOME/titties or ~/.local/share/titties.
func dataDir() (string, error) {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "titties"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "titties"), nil
}

// installedModel returns the voice files when the package is already on disk.
func installedModel(id voiceID) (voice, bool) {
	root, err := dataDir()
	if err != nil {
		return voice{}, false
	}
	dir := filepath.Join(root, id.pkg())
	models, _ := filepath.Glob(filepath.Join(dir, "*.onnx"))
	if len(models) == 0 {
		return voice{}, false
	}
	// sherpa-onnx packages ship their espeak-ng-data; converted Piper voices use the shared copy.
	espeak := filepath.Join(dir, "espeak-ng-data")
	if _, err := os.Stat(espeak); err != nil {
		espeak = filepath.Join(root, "espeak-ng-data")
	}
	return voice{
		model:  models[0],
		tokens: filepath.Join(dir, "tokens.txt"),
		espeak: espeak,
	}, true
}

// autoVoice is the voice name that lets titties pick the voice for a language.
const autoVoice = "auto"

// qualityRank orders voices for auto: medium first (fast and good), then high, low, x_low.
func qualityRank(name string) int {
	for i, q := range []string{"-medium", "-high", "-low", "-x_low"} {
		if strings.HasSuffix(name, q) {
			return i
		}
	}
	return 4
}

// pickVoice chooses the voice for lang among ids: best quality rank, then alphabetical.
func pickVoice(lang string, ids []voiceID) (voiceID, bool) {
	var best voiceID
	found := false
	for _, id := range ids {
		if !strings.EqualFold(id.lang, lang) {
			continue
		}
		if !found || qualityRank(id.name) < qualityRank(best.name) ||
			(qualityRank(id.name) == qualityRank(best.name) && id.name < best.name) {
			best, found = id, true
		}
	}
	return best, found
}

// resolveVoice turns name "auto" into a real voice for lang, preferring one already on disk.
func resolveVoice(lang, name string) (voiceID, error) {
	if name != autoVoice {
		return voiceID{lang: lang, name: name}, nil
	}
	if id, ok := pickVoice(lang, installedVoices()); ok {
		return id, nil
	}
	ids, err := catalog()
	if err != nil {
		return voiceID{}, err
	}
	if id, ok := pickVoice(lang, ids); ok {
		return id, nil
	}
	return voiceID{}, fmt.Errorf("no voice for language %s (see titties list): %w", lang, errNotFound)
}

// installedVoices lists the voices already in the data dir.
func installedVoices() []voiceID {
	root, err := dataDir()
	if err != nil {
		return nil
	}
	dirs, _ := filepath.Glob(filepath.Join(root, pkgPrefix+"*"))
	var ids []voiceID
	for _, d := range dirs {
		if id, ok := parsePkg(filepath.Base(d) + pkgSuffix); ok {
			if _, ok := installedModel(id); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// ensureVoice returns the voice files, downloading the package on first use.
func ensureVoice(id voiceID) (voice, error) {
	root, err := dataDir()
	if err != nil {
		return voice{}, err
	}
	if v, ok := installedModel(id); ok {
		if _, err := os.Stat(v.espeak); err != nil {
			if _, err := ensureEspeakData(root); err != nil {
				return voice{}, err
			}
		}
		return v, nil
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return voice{}, err
	}
	// Extract into a temporary dir and move it in place only when complete, so an interrupted download leaves nothing half-written.
	tmp, err := os.MkdirTemp(root, ".download-")
	if err != nil {
		return voice{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	fmt.Fprintf(os.Stderr, "downloading voice %s %s...\n", id.lang, id.name)
	err = download(releaseURL+id.pkg()+pkgSuffix, tmp)
	if errors.Is(err, errNotFound) {
		// Not repackaged by sherpa-onnx: take it from the official Piper catalog.
		err = installPiperVoice(id, filepath.Join(tmp, id.pkg()))
		if err == nil {
			_, err = ensureEspeakData(root)
		}
	}
	if err != nil {
		return voice{}, fmt.Errorf("voice %s %s (see titties list): %w", id.lang, id.name, err)
	}
	if err := os.Rename(filepath.Join(tmp, id.pkg()), filepath.Join(root, id.pkg())); err != nil {
		return voice{}, err
	}
	v, ok := installedModel(id)
	if !ok {
		return voice{}, fmt.Errorf("voice %s %s: no model in the downloaded package", id.lang, id.name)
	}
	return v, nil
}

// download fetches a .tar.bz2 archive and extracts it into dest.
func download(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx // fixed release URL, one-shot CLI
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("GET %s: %w", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	tr := tar.NewReader(bzip2.NewReader(resp.Body))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name) //nolint:gosec // checked below
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(target, tr); err != nil {
				return err
			}
		}
	}
}

func writeFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil { //nolint:gosec // trusted release archive
		_ = f.Close()
		return err
	}
	return f.Close()
}
