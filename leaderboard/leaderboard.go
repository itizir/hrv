package leaderboard

import (
	"errors"
	"fmt"
	"log"
	"slices"
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

	unorderedHeader = "Master-level players of unknown rank:"
	unorderedPrefix = "- "
)

func Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	msg := ""
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if err := presentModal(s, i); err != nil {
			msg = err.Error()
		}
	case discordgo.InteractionModalSubmit:
		if err := editLeaderboard(s, i); err != nil {
			msg = fmt.Sprintf("Failed to edit leaderboard: %v.", err)
		} else {
			msg = "Leaderboard successfully edited."
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
		return errors.New("failed to fetch message")
	}

	blocks := strings.Split(msg.Content, "\n\n")
	if len(blocks) == 0 {
		return errors.New("empty post")
	}

	rankUnknown := ent.rank == 0

	entries := strings.Split(blocks[0], "\n")
	var (
		added  bool
		update []string
	)
	for _, e := range entries {
		r, n, isID := getRankAndName(e)
		if (isID && n == ent.userID) || n == ent.name {
			if rankUnknown {
				return errors.New("player already ranked")
			}
			continue
		}
		if !rankUnknown && !added && (r >= ent.rank) {
			update = append(update, ent.String())
			added = true
		}
		update = append(update, e)
	}
	if !rankUnknown && !added {
		update = append(update, ent.String())
		added = true
	}
	if added {
		blocks[0] = strings.Join(update, "\n")
	}

	var unorderedEntries []string
	if len(blocks) > 1 && strings.HasPrefix(blocks[1], unorderedHeader) {
		unorderedEntries = strings.Split(blocks[1], "\n")
	} else if rankUnknown {
		unorderedEntries = []string{unorderedHeader}
	}
	ent.rank = 0
	newEntry := unorderedPrefix + ent.String()
	if added {
		unorderedEntries = slices.DeleteFunc(unorderedEntries, func(e string) bool { return e == newEntry })
	} else {
		if slices.Contains(unorderedEntries, newEntry) {
			return errors.New("player already reported as master")
		}
		unorderedEntries = append(unorderedEntries, newEntry)
	}
	if l := len(unorderedEntries); l == 1 {
		blocks = slices.Delete(blocks, 1, 2)
	} else if l > 1 {
		slices.Sort(unorderedEntries[1:])
		unorderedBlock := strings.Join(unorderedEntries, "\n")
		if len(blocks) == 1 {
			blocks = append(blocks, unorderedBlock)
		} else {
			if !strings.HasPrefix(blocks[1], unorderedHeader) {
				blocks = append(blocks, "")
				copy(blocks[2:], blocks[1:])
			}
			blocks[1] = unorderedBlock
		}
	}

	_, err = s.ChannelMessageEdit(thread.ID, thread.ID, strings.Join(blocks, "\n\n"))
	if err != nil {
		log.Printf("failed to edit leaderboard %v: %v", thread.ID, err)
		return errors.New("failed to edit leaderboard message")
	}

	return nil
}
