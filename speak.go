package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	pp "github.com/csukuangfj/piper-phonemize-go/piper_phonemize"
	"github.com/ebitengine/oto/v3"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/v2/sherpa_onnx"
)

// sentencePause is the silence played between two sentences, in seconds.
const sentencePause = 0.25

// speak synthesizes text with the voice and plays it, or saves it to out when set.
func speak(v voice, text string, speed float32, out string) error {
	cfg := sherpa.OfflineTtsConfig{}
	cfg.Model.Vits.Model = v.model
	cfg.Model.Vits.Tokens = v.tokens
	cfg.Model.Vits.NoiseScale = 0.667
	cfg.Model.Vits.NoiseScaleW = 0.8
	cfg.Model.Vits.LengthScale = 1.0
	cfg.Model.NumThreads = min(runtime.NumCPU(), 4)
	cfg.Model.Provider = "cpu"
	cfg.MaxNumSentences = 1

	// Loading the model and phonemizing the text are independent: run them at the same time.
	type phonemes struct {
		sentences [][]int32
		err       error
	}
	ph := make(chan phonemes, 1)
	go func() {
		s, err := phonemize(text, voiceLang(v), v.espeak)
		ph <- phonemes{s, err}
	}()

	tts := sherpa.NewOfflineTts(&cfg)
	if tts == nil {
		return errors.New("cannot load the voice model")
	}
	defer sherpa.DeleteOfflineTts(tts)

	p := <-ph
	if p.err != nil {
		return p.err
	}
	sentences := p.sentences
	gen := func(s [][]int32) *sherpa.GeneratedAudio {
		return tts.GenerateWithConfig("", &sherpa.GenerationConfig{
			SilenceScale:      0.2,
			Speed:             max(speed, 1e-6),
			PhonemeCodepoints: s,
		}, nil)
	}

	if out != "" {
		audio := gen(sentences)
		if audio == nil || len(audio.Samples) == 0 {
			return errors.New("no audio generated")
		}
		if !audio.Save(out) {
			return fmt.Errorf("cannot write %s", out)
		}
		return nil
	}
	return stream(sentences, tts.SampleRate(), gen)
}

// voiceLang reads the espeak-ng language of a voice from its Piper config, so it is known before the model loads.
func voiceLang(v voice) string {
	var conf struct {
		Espeak struct {
			Voice string `json:"voice"`
		} `json:"espeak"`
	}
	if data, err := os.ReadFile(v.model + ".json"); err == nil && json.Unmarshal(data, &conf) == nil && conf.Espeak.Voice != "" {
		return conf.Espeak.Voice
	}
	return "it"
}

// phonemize turns text into one phoneme sequence per sentence.
// sherpa-onnx no longer phonemizes text itself: piper-phonemize does it with espeak-ng.
// espeakDir is the espeak-ng-data shipped with the voice; an empty dir would make the
// library extract its embedded copy into a new temp dir on every run, never removed.
func phonemize(text, lang, espeakDir string) ([][]int32, error) {
	if pp.Initialize(espeakDir) < 0 {
		return nil, errors.New("cannot initialize the phonemizer")
	}
	res := pp.Phonemize(text, lang)
	if res == nil {
		return nil, errors.New("cannot phonemize the text")
	}
	defer pp.DeletePhonemizeResult(res)

	sentences := make([][]int32, 0, res.GetNumSentences())
	for i := range res.GetNumSentences() {
		var s []int32
		for _, cp := range res.GetPhonemes(i) {
			s = append(s, int32(cp))
		}
		if len(s) > 0 {
			sentences = append(sentences, s)
		}
	}
	if len(sentences) == 0 {
		return nil, errors.New("nothing to say after phonemization")
	}
	return sentences, nil
}

// stream plays each sentence as soon as it is synthesized: a goroutine generates
// sentence N+1 while the audio device is still playing sentence N.
func stream(sentences [][]int32, rate int, gen func([][]int32) *sherpa.GeneratedAudio) error {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:      rate,
		ChannelCount:    1,
		Format:          oto.FormatFloat32LE,
		ApplicationName: "titties",
	})
	if err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	<-ready
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("audio: %w", err)
	}

	// The synthesis goroutine writes samples into the pipe, the player reads them as they arrive.
	pr, pw := io.Pipe()
	genErr := make(chan error, 1)
	go func() {
		pause := make([]float32, int(sentencePause*float64(rate)))
		err := func() error {
			for i, s := range sentences {
				audio := gen([][]int32{s})
				if audio == nil || len(audio.Samples) == 0 {
					return errors.New("no audio generated")
				}
				if i > 0 {
					if err := writeSamples(pw, pause); err != nil {
						return err
					}
				}
				if err := writeSamples(pw, audio.Samples); err != nil {
					return err
				}
			}
			// Trailing silence so the device buffer drains the last words before the player stops.
			return writeSamples(pw, pause)
		}()
		_ = pw.CloseWithError(err)
		genErr <- err
	}()

	player := ctx.NewPlayer(pr)
	player.Play()
	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
	}
	err = player.Err()
	_ = pr.Close()
	if gErr := <-genErr; gErr != nil && !errors.Is(gErr, io.ErrClosedPipe) {
		return gErr
	}
	if err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	return nil
}

func writeSamples(w io.Writer, samples []float32) error {
	return binary.Write(w, binary.LittleEndian, samples)
}
