package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// hookInput is the part of a Claude Code Notification hook payload that notify needs.
type hookInput struct {
	NotificationType string `json:"notification_type"`
	Message          string `json:"message"`
	Cwd              string `json:"cwd"`
	TranscriptPath   string `json:"transcript_path"`
}

// phrases says what to speak for each notification type, per language. The first %s is who is
// calling: the named session, or just "a session" when it has no name or the name is too long.
var phrases = map[string]map[string]string{
	"it": {
		"named":              "La sessione %s",
		"unnamed":            "Una sessione",
		"permission_prompt":  "%s ha bisogno del tuo permesso.",
		"permission_tool":    "%s chiede il permesso per usare %s.",
		"idle_prompt":        "%s ti aspetta.",
		"elicitation_dialog": "%s ha una domanda per te.",
		"agent_needs_input":  "%s ha un agente che ha bisogno di te.",
		"":                   "%s ha bisogno di te.",
	},
	"en": {
		"named":              "Session %s",
		"unnamed":            "A session",
		"permission_prompt":  "%s needs your permission.",
		"permission_tool":    "%s asks permission to use %s.",
		"idle_prompt":        "%s is waiting for you.",
		"elicitation_dialog": "%s has a question for you.",
		"agent_needs_input":  "%s has an agent that needs you.",
		"":                   "%s needs you.",
	},
}

// cmdNotify reads a Claude Code Notification hook payload on stdin and says it out loud.
func cmdNotify(args []string) error {
	def, err := loadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	fs := flag.NewFlagSet("notify", flag.ExitOnError)
	lang := fs.String("lang", def.Lang, "voice language")
	name := fs.String("voice", def.Voice, "voice name")
	speed := fs.Float64("speed", def.Speed, "speech speed")
	printOnly := fs.Bool("print", false, "print the sentence instead of saying it")
	_ = fs.Parse(args)

	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		return err
	}
	var in hookInput
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("notify expects a Claude Code hook payload on stdin: %w", err)
	}

	if *printOnly {
		fmt.Println(phrase(*lang, in))
		return nil
	}
	v, err := notifyVoice(*lang, *name)
	if err != nil {
		return err
	}
	return speak(v, phrase(*lang, in), float32(*speed), "")
}

// notifyVoice loads the configured voice; a voice that does not exist for the language
// (a mismatched pair in the plugin settings) falls back to auto rather than staying silent.
func notifyVoice(lang, name string) (voice, error) {
	id, err := resolveVoice(lang, name)
	if err == nil {
		var v voice
		if v, err = ensureVoice(id); err == nil {
			return v, nil
		}
	}
	if name == autoVoice || !errors.Is(err, errNotFound) {
		return voice{}, err
	}
	fmt.Fprintf(os.Stderr, "titties: no voice %s for %s, using auto\n", name, lang)
	if id, err = resolveVoice(lang, autoVoice); err != nil {
		return voice{}, err
	}
	return ensureVoice(id)
}

// maxDetail caps the spoken detail, in characters: a notification should stay short.
const maxDetail = 220

var permissionTool = regexp.MustCompile(`permission to use (\S+)`)

// phrase says what Claude needs, in the voice's language (English as fallback), followed by the detail of what for.
func phrase(lang string, in hookInput) string {
	table, ok := phrases[strings.ToLower(strings.SplitN(lang, "_", 2)[0])]
	if !ok {
		table = phrases["en"]
	}
	last := lastAssistant(in.TranscriptPath)
	session := table["unnamed"]
	if name := last.sessionName(); name != "" {
		session = fmt.Sprintf(table["named"], name)
	}

	var head, detail string
	switch in.NotificationType {
	case "permission_prompt":
		head = fmt.Sprintf(table["permission_prompt"], session)
		if m := permissionTool.FindStringSubmatch(in.Message); m != nil {
			head = fmt.Sprintf(table["permission_tool"], session, m[1])
		}
		detail = last.toolDescription
	default:
		text, ok := table[in.NotificationType]
		if !ok {
			text = table[""]
		}
		head = fmt.Sprintf(text, session)
		detail = lastQuestion(last.text)
	}
	if detail = clip(cleanMarkdown(detail), maxDetail); detail == "" {
		return head
	}
	return head + " " + detail
}

type assistantTurn struct {
	text            string // last text block written by Claude
	toolDescription string // description of the last tool call, e.g. what a Bash command does

	// Session names, as Claude Code writes them in the transcript (not a documented format).
	customTitle string // the name the user gave the session
	agentName   string // the name of a background session
	aiTitle     string // the title Claude Code generated
}

// maxSessionName is the longest session name worth saying; a longer one is just "a session".
const maxSessionName = 30

// sessionName is the name the user gave the session, or "" when it has none or it is too long
// to say. A background session's agent name counts only when it is not the generated title.
func (t assistantTurn) sessionName() string {
	name := strings.TrimSpace(t.customTitle)
	if name == "" && t.agentName != t.aiTitle {
		name = strings.TrimSpace(t.agentName)
	}
	if utf8.RuneCountInString(name) > maxSessionName {
		return ""
	}
	return name
}

// lastAssistant reads the end of the session transcript (JSONL, one content block per line)
// and returns Claude's last text and the description of its last tool call.
func lastAssistant(path string) assistantTurn {
	var turn assistantTurn
	if path == "" {
		return turn
	}
	f, err := os.Open(path) //nolint:gosec // path comes from Claude Code
	if err != nil {
		return turn
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		// Decode only the lines that matter: a transcript can be tens of MB.
		if !bytes.Contains(line, []byte(`"type":"assistant"`)) && !bytes.Contains(line, []byte(`Title"`)) &&
			!bytes.Contains(line, []byte(`"agentName"`)) {
			continue
		}
		var entry struct {
			Type        string `json:"type"`
			CustomTitle string `json:"customTitle"`
			AgentName   string `json:"agentName"`
			AITitle     string `json:"aiTitle"`
			Message     struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		switch entry.Type {
		case "custom-title":
			turn.customTitle = entry.CustomTitle
			continue
		case "agent-name":
			turn.agentName = entry.AgentName
			continue
		case "ai-title":
			turn.aiTitle = entry.AITitle
			continue
		case "assistant":
		default:
			continue
		}
		var blocks []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			Input struct {
				Description string `json:"description"`
			} `json:"input"`
		}
		if json.Unmarshal(entry.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if strings.TrimSpace(b.Text) != "" {
					turn.text = b.Text
				}
			case "tool_use":
				turn.toolDescription = b.Input.Description
			}
		}
	}
	return turn
}

// lastQuestion returns the last question in text, or its last sentence when there is none.
func lastQuestion(text string) string {
	sentences := splitSentences(text)
	for i := len(sentences) - 1; i >= 0; i-- {
		if strings.HasSuffix(sentences[i], "?") {
			return sentences[i]
		}
	}
	if len(sentences) == 0 {
		return ""
	}
	return sentences[len(sentences)-1]
}

// splitSentences cuts after . ! ? only when a space or the end follows, so example.com
// or v0.1.3 stay whole; a line break always ends a sentence.
func splitSentences(text string) []string {
	var out []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		r := []rune(line)
		start := 0
		for i := 0; i < len(r); i++ {
			if r[i] != '.' && r[i] != '!' && r[i] != '?' {
				continue
			}
			if i+1 == len(r) || r[i+1] == ' ' || r[i+1] == '\t' {
				add(string(r[start : i+1]))
				start = i + 1
			}
		}
		add(string(r[start:]))
	}
	return out
}

var (
	mdLink  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdNoise = regexp.MustCompile("[`*_#>|]+")
	urls    = regexp.MustCompile(`https?://\S+`)
	spaces  = regexp.MustCompile(`\s+`)
)

// cleanMarkdown keeps the words a voice should read: no links, URLs, backticks or emphasis.
func cleanMarkdown(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = urls.ReplaceAllString(s, "")
	s = strings.TrimPrefix(strings.TrimSpace(s), "- ")
	s = mdNoise.ReplaceAllString(s, " ")
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

// clip cuts s at a word boundary so it stays under max characters.
func clip(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	r := []rune(s)[:maxLen]
	if i := strings.LastIndexByte(string(r), ' '); i > 0 {
		return string(r)[:i] + "..."
	}
	return string(r) + "..."
}
