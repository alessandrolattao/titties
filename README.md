# TitTieS

Says text out loud, fully offline, with a local [Piper](https://github.com/OHF-Voice/piper1-gpl) voice run through [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx).

```sh
titties say "ciao, oggi è una bella giornata"
titties say -lang it_IT -voice miro-high "ciao, io sono Miro"
titties say -o out.wav "save it instead of playing it"
titties list            # every Piper voice, with the installed ones marked
titties list -lang it   # only the Italian ones
titties voice           # pick language and voice interactively, hearing each one
titties default                     # show the defaults say uses
titties default -voice miro-high    # change them, no flag needed on say anymore
```

Defaults live in `~/.config/titties/config.json` (`lang`, `voice`, `speed`); a flag on `say` overrides them for that call only.

## Claude Code plugin

TitTieS is also a Claude Code plugin: Claude calls you out loud when it needs your permission or an answer, and says who and what for: "La sessione titties chiede il permesso per usare Bash. Push the branch", or "Una sessione ti aspetta. Quale licenza vuoi?" when the session has no name or one longer than 30 characters.

```
/plugin install titties --marketplace alessandrolattao/titties
```

or, step by step:

```
/plugin marketplace add alessandrolattao/titties
/plugin install titties@titties
```

The plugin is silent until you choose who may call you: run `/titties:on` in a session (for example the one that hands out work to the others) and only that session speaks; `/titties:off` silences it again, and the mark goes away when the session ends.

It speaks when Claude asks a permission or a multiple-choice question, never just because a session went idle. A skill that decides itself when you are needed, such as the release-manager, turns itself on with `titties:auto-on` and calls you with its own sentence through `titties:call`, queued like every other voice.

The plugin speaks with the voice you pick with `titties voice` in a terminal (or ask Claude for `/titties:voice`). On the first session the plugin downloads the TitTieS release matching its version, and the voice, into its data dir, and links `titties` into `~/.local/bin` when it is not installed already; nothing else is needed. Linux and macOS only for now.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/alessandrolattao/titties/main/install.sh | sh
```

Linux and macOS, amd64 and arm64. Windows: download the zip from the [releases](https://github.com/alessandrolattao/titties/releases).

From source, needs Go with cgo (a C compiler). Audio is played directly through [oto](https://github.com/ebitengine/oto): PulseAudio/PipeWire or ALSA on Linux, CoreAudio on macOS, WASAPI on Windows. Synthesis is tested in CI on Linux, macOS and Windows; playback only on Linux so far.

```sh
go install github.com/alessandrolattao/titties@latest
```

## Voices

`titties list` shows about 220 voices in 61 languages: the ones sherpa-onnx repackages, plus the official [Piper voices](https://huggingface.co/rhasspy/piper-voices) it does not (`it_IT` `serena`, Japanese, Korean, Hebrew, ...), which titties converts on download the way sherpa-onnx does. The default is `it_IT` `paola-medium`; `-voice auto` picks a voice for the language.

A voice is downloaded from the sherpa-onnx `tts-models` release on first use and kept in `$XDG_DATA_HOME/titties` (`~/.local/share/titties`).

## How it works

Text → phonemes with [piper-phonemize-go](https://github.com/csukuangfj/piper-phonemize-go) (embedded espeak-ng data), one sequence per sentence → Piper VITS model in sherpa-onnx on CPU → raw audio streamed to the sound card with oto.

Synthesis and playback run together: a goroutine generates the next sentence while the current one is playing, so a long text starts speaking after the first sentence instead of after the whole text. With `-o` the whole text is generated at once and saved.

## License

TitTieS is licensed under the [GNU General Public License v3.0](LICENSE). It links [espeak-ng](https://github.com/espeak-ng/espeak-ng) (GPL-3.0) through piper-phonemize.

The voices are not part of this repository: each one is downloaded on first use and keeps its own license, stated in the `MODEL_CARD` or `README.md` inside its folder under `~/.local/share/titties`. Some are non-commercial (for example `it_IT` `miro-high` is CC BY-NC-SA 4.0), so check the voice before using its audio commercially.
