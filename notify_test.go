package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPhrase(t *testing.T) {
	transcript := writeTranscript(t,
		`{"type":"user","message":{"content":"ciao"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Ho finito la **build**.\n\nQuale licenza vuoi? Ti consiglio [GPL](https://x.y). Dimmi tu."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"git push","description":"Push the branch"}}]}}`,
		`not json`,
		`{"type":"ai-title","aiTitle":"ricerca modelli tts","sessionId":"x"}`,
		`{"type":"custom-title","customTitle":"titties","sessionId":"x"}`,
	)
	untitled := writeTranscript(t,
		`{"type":"ai-title","aiTitle":"ci build failures","sessionId":"y"}`,
		`{"type":"agent-name","agentName":"ci build failures","sessionId":"y"}`,
	)
	longName := writeTranscript(t,
		`{"type":"custom-title","customTitle":"ricerca modelli tts veloci e confronto prezzi","sessionId":"w"}`,
	)
	background := writeTranscript(t,
		`{"type":"ai-title","aiTitle":"ci build failures","sessionId":"z"}`,
		`{"type":"agent-name","agentName":"nightly","sessionId":"z"}`,
	)

	tests := []struct {
		name string
		lang string
		in   hookInput
		want string
	}{
		{
			name: "permission says the tool and what it does",
			lang: "it_IT",
			in:   hookInput{NotificationType: "permission_prompt", Message: "Claude needs your permission to use Bash", Cwd: "/p/titties", TranscriptPath: transcript},
			want: "La sessione titties chiede il permesso per usare Bash. Push the branch",
		},
		{
			name: "idle reads the last question, without markdown",
			lang: "it_IT",
			in:   hookInput{NotificationType: "idle_prompt", Cwd: "/p/other", TranscriptPath: transcript},
			want: "La sessione titties ti aspetta. Quale licenza vuoi?",
		},
		{
			name: "no name: just a session, the generated title does not count",
			lang: "it_IT",
			in:   hookInput{NotificationType: "idle_prompt", Cwd: "/p/blabla", TranscriptPath: untitled},
			want: "Una sessione ti aspetta.",
		},
		{
			name: "name longer than 30 characters: just a session",
			lang: "it_IT",
			in:   hookInput{NotificationType: "idle_prompt", Cwd: "/p/blabla", TranscriptPath: longName},
			want: "Una sessione ti aspetta.",
		},
		{
			name: "background session named by the user",
			lang: "it_IT",
			in:   hookInput{NotificationType: "idle_prompt", Cwd: "/p/blabla", TranscriptPath: background},
			want: "La sessione nightly ti aspetta.",
		},
		{
			name: "no transcript: just a session",
			lang: "it_IT",
			in:   hookInput{NotificationType: "idle_prompt", Cwd: "/p/my-app"},
			want: "Una sessione ti aspetta.",
		},
		{
			name: "unknown language falls back to English",
			lang: "de_DE",
			in:   hookInput{NotificationType: "elicitation_dialog", Cwd: "/p/app"},
			want: "A session has a question for you.",
		},
		{
			name: "unknown type and no cwd",
			lang: "it_IT",
			in:   hookInput{NotificationType: "something_new"},
			want: "Una sessione ha bisogno di te.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := phrase(tt.lang, tt.in); got != tt.want {
				t.Errorf("phrase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLastQuestionWithoutQuestion(t *testing.T) {
	if got := lastQuestion("Fatto. Tutto verde!"); got != "Tutto verde!" {
		t.Errorf("got %q", got)
	}
}

func TestClip(t *testing.T) {
	if got := clip("uno due tre quattro", 9); got != "uno due..." {
		t.Errorf("got %q", got)
	}
}

func TestSplitSentencesKeepsDottedWords(t *testing.T) {
	got := splitSentences("Ho controllato example.com e la v0.1.3. Va bene? Sì!\nNuova riga")
	want := []string{"Ho controllato example.com e la v0.1.3.", "Va bene?", "Sì!", "Nuova riga"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitSentences = %q, want %q", got, want)
	}
}
