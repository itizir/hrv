package countvotes

import "github.com/bwmarrin/discordgo"

const (
	emojiPlayed = "Genmat"
	emojiMain   = "HRV"
)

var (
	emojiSecondary = []string{
		"Fun",
		"Brutal",
		"Ingenious",
		"Artistic",
	}

	knownEmojis = map[string]*discordgo.Emoji{}
)

func init() {
	for _, s := range append(emojiSecondary, emojiPlayed, emojiMain) {
		knownEmojis[s] = nil
	}
}
