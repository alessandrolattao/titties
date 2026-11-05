package main

import "testing"

func TestSamplePhrase(t *testing.T) {
	tests := map[voiceID]string{
		{"it_IT", "paola-medium"}:         "Ciao, sono Paola.",
		{"de_DE", "thorsten-high"}:        "Hallo, ich bin Thorsten.",
		{"ja_JP", "hi_fi_captain-medium"}: "こんにちは、Hi fi captainです。",
		{"xx_XX", "nobody-low"}:           "Hi, I am Nobody.",
	}
	for id, want := range tests {
		if got := samplePhrase(id); got != want {
			t.Errorf("samplePhrase(%v) = %q, want %q", id, got, want)
		}
	}
}

// Every language family in the catalog has its own phrase.
func TestHelloPhrasesCoverCatalog(t *testing.T) {
	families := []string{"ar", "bg", "bn", "ca", "cs", "cy", "da", "de", "el", "en", "es", "et", "eu", "fa", "fi", "fr", "he", "hi", "hu", "hy", "id", "is", "it", "ja", "ka", "kk", "ko", "ku", "lb", "lt", "lv", "ml", "mr", "ne", "nl", "no", "pl", "pt", "ro", "ru", "sk", "sl", "sq", "sr", "sv", "sw", "te", "th", "tr", "uk", "ur", "vi", "zh"}
	for _, f := range families {
		if _, ok := helloPhrases[f]; !ok {
			t.Errorf("no hello phrase for %s", f)
		}
	}
}
