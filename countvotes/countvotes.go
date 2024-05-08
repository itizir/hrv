package countvotes

import (
	"errors"
	"fmt"
	"log"
	"strings"

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

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Println("failed to respond to interactions:", err)
		return nil
	}

	resp := determineResults(s, guildID, chanID)

	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &resp})
	if err != nil {
		log.Println("failed to edit response:", err)
	}

	return nil
}

func determineResults(s *discordgo.Session, guildID, chanID string) string {
	posts, err := fetchPosts(s, guildID, chanID)
	if err != nil {
		return fmt.Sprintf("Oops! Failed to get the data from <#%v>: %v", chanID, err)
	}

	resp := ""
	con := contest{posts: posts}

	if irregularities := con.validate(); len(irregularities) > 0 {
		resp += "Oh no! Found some irregularities:\n"
		resp += "- " + strings.Join(irregularities, "\n- ") + "\n"
		resp += "...so the results shouldn't be trusted. 😿\n\n"
	}

	win := con.winners()
	if l := len(win); l == 0 {
		resp += fmt.Sprintf("Ooof! Could not determine _any_ winners in <#%v>!", chanID)
		return resp
	} else if l < 1+len(emojiSecondary) {
		resp += "Could not determine winners for all categories: undecidable ties, or too few eligible submissions. Sad. Well, anyway...\n\n"
	}

	resp += fmt.Sprintf("🥁 Without further ado, the winners of <#%s>:\n", chanID)
	resp += "- " + strings.Join(win, "\n- ") + "\n"
	resp += "Congratulations! 🎉"

	return resp
}
