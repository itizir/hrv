package leaderboard

import (
	"errors"
	"fmt"
	"os"

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
		nameOrMention, _ := vals[adminCommandArgKeyName].(string)
		if nameOrMention != "" {
			id, err := findUserID(s, i.GuildID, nameOrMention)
			if err == nil && id != "" {
				nameOrMention = userMention(id)
			}
		}
		rank, _ := vals[adminCommandArgKeyRank].(float64)
		season, ok := vals[adminCommandArgKeySeason].(float64)
		if !ok {
			season = -1
		}
		if err := deleteEntry(s, i, nameOrMention, int(rank), int(season)); err != nil {
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

func deleteEntry(s *discordgo.Session, i *discordgo.InteractionCreate, nameOrMention string, rank, season int) error {
	if nameOrMention == "" && rank < 1 {
		return errors.New("need at least name or rank")
	}

	thread, _, err := getSeasonThread(s, i.GuildID, i.AppID, season)
	if err != nil {
		return err
	}

	if !yuckyMutex.TryLock() {
		return fmt.Errorf("%s is currently busy, sorry; try again", userMention(i.AppID))
	}
	defer yuckyMutex.Unlock()

	ld, err := getLeaderboardData(s, thread)
	if err != nil {
		return err
	}

	ld.removeEntries(nameOrMention, rank)

	return ld.updateMessages(s)
}
