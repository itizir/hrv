package leaderboard

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	ApplicationCommand = &discordgo.ApplicationCommand{
		Type:        discordgo.ChatApplicationCommand,
		Name:        "rank",
		Description: "Report player rank for leaderboard",
	}
)

const (
	modalKeyRank       = "rank"
	modalKeyRankPoints = "rank_points"
	modalKeyPlayer     = "player"
	modalKeySeason     = "season"
)

func Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		return presentModal(s, i)
	case discordgo.InteractionModalSubmit:
		msg := "Leaderboard successfully edited."
		err := editLeaderboard(s, i)
		if err != nil {
			msg = fmt.Sprintf("Failed to edit leaderboard: %v.", err)
		}
		return s.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
	}
	return fmt.Errorf("unhandled interaction type: %v", i.Type)
}

func editLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	ent, season, err := parseModalInput(s, i)
	if err != nil {
		return err
	}

	thread, _, err := getSeasonThread(s, i.GuildID, i.AppID, season)
	if err != nil {
		return err
	}
	msg, err := s.ChannelMessage(thread.ID, thread.ID)
	if err != nil {
		log.Printf("failed to fetch message %v: %v", thread.ID, err)
		return fmt.Errorf("failed to fetch message")
	}

	entries := strings.Split(msg.Content, "\n")
	var (
		added  bool
		update []string
	)
	for _, e := range entries {
		r, n, isID := partialParsing(e)
		if (isID && n == ent.userID) || n == ent.name {
			continue
		}
		if !added && r >= ent.rank {
			update = append(update, ent.String())
			added = true
		}
		update = append(update, e)
	}
	if !added {
		update = append(update, ent.String())
	}

	_, err = s.ChannelMessageEdit(thread.ID, thread.ID, strings.Join(update, "\n"))
	if err != nil {
		log.Printf("failed to edit leaderboard %v: %v", thread.ID, err)
		return fmt.Errorf("failed to edit leaderboard message")
	}

	return nil
}
