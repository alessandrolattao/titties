// Command titties speaks text out loud with a local Piper voice.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

const usage = `usage:
  titties say [-lang it_IT] [-voice paola-medium] [-speed 1.0] [-o out.wav] "text"
  titties list [-lang it]
  titties voice                                     pick language and voice interactively
  titties default                                   show the defaults used by say
  titties default [-lang it_IT] [-voice miro-high] [-speed 1.2]   change them
  titties notify [-lang it_IT] [-voice paola-medium] < hook.json   speak a Claude Code Notification hook
  titties version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "say":
		err = cmdSay(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "default":
		err = cmdDefault(os.Args[2:])
	case "voice":
		err = cmdVoice(os.Args[2:])
	case "notify":
		err = cmdNotify(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("titties", version)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "titties:", err)
		os.Exit(1)
	}
}

func cmdSay(args []string) error {
	def, err := loadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	fs := flag.NewFlagSet("say", flag.ExitOnError)
	lang := fs.String("lang", def.Lang, "voice language, as shown by titties list")
	name := fs.String("voice", def.Voice, "voice name, as shown by titties list, or auto")
	speed := fs.Float64("speed", def.Speed, "speech speed, larger is faster")
	out := fs.String("o", "", "write the audio to this WAV file instead of playing it")
	_ = fs.Parse(args)

	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "" {
		return fmt.Errorf("nothing to say\n\n%s", usage)
	}

	id, err := resolveVoice(*lang, *name)
	if err != nil {
		return err
	}
	v, err := ensureVoice(id)
	if err != nil {
		return err
	}
	return speak(v, text, float32(*speed), *out)
}

// cmdDefault shows the defaults used by say, or changes the ones given as flags.
func cmdDefault(args []string) error {
	c, err := loadConfig()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	fs := flag.NewFlagSet("default", flag.ExitOnError)
	fs.StringVar(&c.Lang, "lang", c.Lang, "default voice language")
	fs.StringVar(&c.Voice, "voice", c.Voice, "default voice name")
	fs.Float64Var(&c.Speed, "speed", c.Speed, "default speech speed")
	_ = fs.Parse(args)

	if fs.NFlag() > 0 {
		if c.Speed <= 0 {
			return errors.New("speed must be greater than 0")
		}
		// Downloading the voice now checks that it exists and makes the next say fast.
		id, err := resolveVoice(c.Lang, c.Voice)
		if err != nil {
			return err
		}
		if _, err := ensureVoice(id); err != nil {
			return err
		}
		path, err := saveConfig(c)
		if err != nil {
			return err
		}
		fmt.Println("saved", path)
	}
	fmt.Printf("lang   %s\nvoice  %s\nspeed  %g\n", c.Lang, c.Voice, c.Speed)
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	lang := fs.String("lang", "", "only voices whose language starts with this, e.g. it or it_IT")
	_ = fs.Parse(args)

	ids, err := catalog()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "LANG\tVOICE\tINSTALLED")
	for _, id := range ids {
		if !strings.HasPrefix(strings.ToLower(id.lang), strings.ToLower(*lang)) {
			continue
		}
		installed := ""
		if _, ok := installedModel(id); ok {
			installed = "yes"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", id.lang, id.name, installed)
	}
	return w.Flush()
}
