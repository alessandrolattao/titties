package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/huh/v2"
)

// cmdVoice lets the user pick the default language and voice interactively:
// the voice list only shows the voices of the chosen language, and each pick is played before saving.
func cmdVoice(_ []string) error {
	c, err := loadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	fmt.Fprintln(os.Stderr, "loading the voice catalog...")
	ids, err := catalog()
	if err != nil {
		return err
	}
	installed := make(map[voiceID]bool)
	for _, id := range installedVoices() {
		installed[id] = true
	}

	byLang := make(map[string][]voiceID)
	for _, id := range ids {
		byLang[id.lang] = append(byLang[id.lang], id)
	}
	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	slices.SortFunc(langs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })

	langOptions := make([]huh.Option[string], 0, len(langs))
	for _, l := range langs {
		langOptions = append(langOptions, huh.NewOption(fmt.Sprintf("%s (%d)", l, len(byLang[l])), l))
	}

	lang, name := c.Lang, c.Voice
	if _, ok := byLang[lang]; !ok {
		lang = "it_IT"
	}
	for {
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Language").
				Description("type / to search").
				Options(langOptions...).
				Filtering(false).
				Height(12).
				Value(&lang),
			huh.NewSelect[string]().
				TitleFunc(func() string { return "Voice for " + lang }, &lang).
				Description("✓ already downloaded").
				OptionsFunc(func() []huh.Option[string] {
					opts := make([]huh.Option[string], 0, len(byLang[lang]))
					for _, id := range byLang[lang] {
						label := id.name
						if installed[id] {
							label += " ✓"
						}
						opts = append(opts, huh.NewOption(label, id.name))
					}
					return opts
				}, &lang).
				Height(10).
				Value(&name),
		))
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr, "nothing changed")
				return nil
			}
			return err
		}

		id := voiceID{lang: lang, name: name}
		v, err := ensureVoice(id)
		if err != nil {
			return err
		}
		installed[id] = true
		if err := speak(v, samplePhrase(id), float32(c.Speed), ""); err != nil {
			return err
		}

		keep := true
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("Use %s %s?", lang, name)).
			Affirmative("Save").
			Negative("Pick another").
			Value(&keep)
		if err := confirm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr, "nothing changed")
				return nil
			}
			return err
		}
		if !keep {
			continue
		}

		c.Lang, c.Voice = lang, name
		path, err := saveConfig(c)
		if err != nil {
			return err
		}
		fmt.Printf("saved %s %s in %s\n", lang, name, path)
		return nil
	}
}

// helloPhrases is "Hello, I am <name>." in each language family of the catalog.
var helloPhrases = map[string]string{
	"ar": "مرحبا، أنا %s.",
	"bg": "Здравей, аз съм %s.",
	"bn": "নমস্কার, আমি %s।",
	"ca": "Hola, sóc %s.",
	"cs": "Ahoj, jsem %s.",
	"cy": "Helo, %s ydw i.",
	"da": "Hej, jeg hedder %s.",
	"de": "Hallo, ich bin %s.",
	"el": "Γεια σου, είμαι %s.",
	"en": "Hi, I am %s.",
	"es": "Hola, soy %s.",
	"et": "Tere, mina olen %s.",
	"eu": "Kaixo, %s naiz.",
	"fa": "سلام، من %s هستم.",
	"fi": "Hei, olen %s.",
	"fr": "Bonjour, je suis %s.",
	"he": "שלום, אני %s.",
	"hi": "नमस्ते, मैं %s हूँ।",
	"hu": "Szia, %s vagyok.",
	"hy": "Բարև, ես %s եմ։",
	"id": "Halo, saya %s.",
	"is": "Halló, ég heiti %s.",
	"it": "Ciao, sono %s.",
	"ja": "こんにちは、%sです。",
	"ka": "გამარჯობა, მე ვარ %s.",
	"kk": "Сәлем, мен %s.",
	"ko": "안녕하세요, 저는 %s입니다.",
	"ku": "Silav, ez %s im.",
	"lb": "Moien, ech sinn %s.",
	"lt": "Labas, aš esu %s.",
	"lv": "Sveiki, es esmu %s.",
	"ml": "നമസ്കാരം, ഞാൻ %s ആണ്.",
	"mr": "नमस्कार, मी %s आहे.",
	"ne": "नमस्ते, म %s हुँ।",
	"nl": "Hallo, ik ben %s.",
	"no": "Hei, jeg heter %s.",
	"pl": "Cześć, jestem %s.",
	"pt": "Olá, eu sou %s.",
	"ro": "Bună, sunt %s.",
	"ru": "Привет, я %s.",
	"sk": "Ahoj, som %s.",
	"sl": "Živjo, jaz sem %s.",
	"sq": "Përshëndetje, unë jam %s.",
	"sr": "Здраво, ја сам %s.",
	"sv": "Hej, jag heter %s.",
	"sw": "Habari, mimi ni %s.",
	"te": "నమస్కారం, నేను %s.",
	"th": "สวัสดี ฉันชื่อ %s",
	"tr": "Merhaba, ben %s.",
	"uk": "Привіт, я %s.",
	"ur": "السلام علیکم، میں %s ہوں۔",
	"vi": "Xin chào, tôi là %s.",
	"zh": "你好，我是%s。",
}

// samplePhrase is what a voice says when picked: "Hello, I am <name>." in its own language.
func samplePhrase(id voiceID) string {
	who := strings.ReplaceAll(strings.SplitN(id.name, "-", 2)[0], "_", " ")
	if who != "" {
		who = strings.ToUpper(who[:1]) + who[1:]
	}
	text, ok := helloPhrases[strings.ToLower(strings.SplitN(id.lang, "_", 2)[0])]
	if !ok {
		text = helloPhrases["en"]
	}
	return fmt.Sprintf(text, who)
}
