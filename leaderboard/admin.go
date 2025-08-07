package leaderboard

import (
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	ApplicationAdminCommand = &discordgo.ApplicationCommand{
		Type:        discordgo.ChatApplicationCommand,
		Name:        "rank_admin",
		Description: "Admin functions for leaderboard",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        adminCommandDelete,
				Description: "Remove a player rank entry from leaderboard",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        adminCommandArgKeyName,
						Description: "Player name (prefix-matching)",
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        adminCommandArgKeyRank,
						Description: "Player rank",
					},
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        adminCommandArgKeySeason,
						Description: "Season number, defaults to latest",
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        adminCommandStartSeason,
				Description: "Start a new season",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        adminCommandArgKeyName,
						Description: fmt.Sprintf("Season name, needs to start with %q", threadNamePrefix),
						Required:    true,
					},
				},
			},
		},
	}

	// export for easy override in testing environment
	AdminID = os.Getenv("LEADERBOARD_ADMIN_ID")
)

const (
	adminCommandDelete      = "delete"
	adminCommandStartSeason = "start"

	adminCommandArgKeyName   = "name"
	adminCommandArgKeyRank   = "rank"
	adminCommandArgKeySeason = "season"
)

func HandleAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	// for now insist it can only be me. better solution would be role-based enforced in Guild settings instead of done here in the handler
	if i.Member == nil || i.Member.User.ID != AdminID {
		return s.InteractionRespond(i.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Unauthorised. Ask %s for help!", userMention(AdminID)),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
	}

	if l := len(i.ApplicationCommandData().Options); l != 1 {
		return fmt.Errorf("invalid options length in %s: %d", ApplicationAdminCommand.Name, l)
	}
	o := i.ApplicationCommandData().Options[0]

	msg := "Not yet implemented!"
	switch o.Name {
	case adminCommandDelete:
		vals := optionsToDict(o.Options)
		name, _ := vals[adminCommandArgKeyName].(string)
		if name != "" {
			id, err := findUserID(s, i.GuildID, name)
			if err == nil && id != "" {
				name = userMention(id)
			}
		}
		rank, _ := vals[adminCommandArgKeyRank].(float64)
		season, ok := vals[adminCommandArgKeySeason].(float64)
		if !ok {
			season = -1
		}
		if err := deleteEntry(s, i, name, int(rank), int(season)); err != nil {
			msg = err.Error()
		} else {
			msg = "OK!"
		}
	case adminCommandStartSeason:
		vals := optionsToDict(o.Options)
		name, _ := vals[adminCommandArgKeyName].(string)
		if err := createSeasonThread(s, i.GuildID, i.AppID, name); err != nil {
			msg = err.Error()
		} else {
			msg = "OK!"
		}
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

func optionsToDict(opts []*discordgo.ApplicationCommandInteractionDataOption) map[string]any {
	vals := make(map[string]any)
	for _, o := range opts {
		vals[o.Name] = o.Value
	}
	return vals
}

func deleteEntry(s *discordgo.Session, i *discordgo.InteractionCreate, name string, rank, season int) error {
	if name == "" && rank < 1 {
		return errors.New("need at least name or rank")
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

	blockNumber := 0
	match := func(s string) bool {
		if s == "" {
			blockNumber++
			return false
		}
		switch blockNumber {
		case 0:
			sep := "\\. "
			i := strings.Index(s, sep)
			if i > 0 {
				s = s[i+len(sep):]
			}
			return strings.HasPrefix(s, name)
		case 1:
			return strings.HasPrefix(s, unorderedPrefix+name)
		default:
			return false
		}
	}
	if rank > 0 {
		m := fmt.Sprintf("%d\\. %s", rank, name)
		match = func(s string) bool {
			if blockNumber > 0 {
				return false
			}
			if s == "" {
				blockNumber++
				return false
			}
			return strings.HasPrefix(s, m)
		}
	}

	entries := strings.Split(msg.Content, "\n")
	update := slices.DeleteFunc(entries, match)
	content := strings.Join(update, "\n")
	// clean up if laste unordered entry was deleted
	content = strings.Replace(content, unorderedHeader+"\n\n", "", 1)
	content = strings.TrimSuffix(content, "\n\n"+unorderedHeader)

	_, err = s.ChannelMessageEdit(thread.ID, thread.ID, content)
	if err != nil {
		log.Printf("failed to edit leaderboard %v: %v", thread.ID, err)
		return errors.New("failed to edit leaderboard message")
	}

	return nil
}
