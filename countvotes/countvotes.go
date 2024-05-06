package countvotes

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var (
	ApplicationCommand = &discordgo.ApplicationCommand{
		Name:        "countvotes",
		Description: "Count votes and determine contest winners",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionChannel,
				Name:         "channel",
				Description:  "Forum channel of the contest",
				Required:     true,
				Autocomplete: true,
				ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildForum},
			},
		},
	}
)

func Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	guildID := i.GuildID
	opts := i.ApplicationCommandData().Options
	if len(opts) != 1 {
		return errors.New("invalid number of arguments")
	}
	chanID, _ := opts[0].Value.(string)
	if chanID == "" {
		return errors.New("invalid argument value")
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Coming right up, calculating winners of <#%s>!", chanID),
		},
	})
	s.ChannelTyping(i.ChannelID)

	posts, err := fetchPosts(s, guildID, chanID)
	if err != nil {
		s.ChannelMessageSend(i.ChannelID, fmt.Sprintf("Oops! Failed to do so: %v", err))
		return nil
	}

	con := contest{posts: posts}

	if irregularities := con.validate(); len(irregularities) > 0 {
		s.ChannelMessageSend(i.ChannelID, "Oh no! Found some irregularities:")
		for _, irr := range irregularities {
			s.ChannelMessageSend(i.ChannelID, irr)
		}
		s.ChannelMessageSend(i.ChannelID, "...so the results shouldn't be trusted. 😿")
	}

	win := con.winners()
	if l := len(win); l == 0 {
		s.ChannelMessageSend(i.ChannelID, "Ooof! Could not determine any winners!")
	} else if l < 1+len(emojiSecondary) {
		s.ChannelMessageSend(i.ChannelID, "Could not determine winners for all categories... Undecidable ties, or too few eligible submissions. Sad.")
	}

	resp := fmt.Sprintf("...without further ado, the winners of <#%s>:\n\n", chanID)
	for _, w := range win {
		resp += fmt.Sprintf("%v\n\n", w)
	}
	resp = resp[:len(resp)-2]

	s.ChannelMessageSend(i.ChannelID, resp)

	return nil
}
