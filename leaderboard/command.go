package leaderboard

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
)

var (
	ApplicationCommand = &discordgo.ApplicationCommand{
		Type:        discordgo.ChatApplicationCommand,
		Name:        "rank",
		Description: "Report player rank for leaderboard",
	}

	// the fetch messages-update messages operation is very much non-atomic, so races could be bad
	// don't want to go overboard trying to synchronise more elegantly...
	yuckyMutex sync.Mutex
)

func Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	msg := ""
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if err := presentModal(s, i); err != nil {
			msg = err.Error()
		}
	case discordgo.InteractionModalSubmit:
		if id, err := editLeaderboard(s, i); err != nil {
			msg = fmt.Sprintf("Failed to edit leaderboard: %v.", err)
		} else {
			msg = fmt.Sprintf("Leaderboard %s successfully edited.", channelMention(id))
		}
	default:
		return fmt.Errorf("unhandled interaction type: %v", i.Type)
	}

	if msg != "" {
		return s.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
	}

	return nil
}

func userMention(id string) string {
	return (&discordgo.User{ID: id}).Mention()
}

func channelMention(id string) string {
	return (&discordgo.Channel{ID: id}).Mention()
}
