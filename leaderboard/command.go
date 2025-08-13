package leaderboard

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var (
	ApplicationCommand = &discordgo.ApplicationCommand{
		Type:        discordgo.ChatApplicationCommand,
		Name:        "rank",
		Description: "Report player rank for leaderboard",
	}
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
