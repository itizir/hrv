package leaderboard

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func presentModal(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	_, currentSeason, err := getSeasonThread(s, i.GuildID, i.AppID, -1)
	if err != nil {
		return err
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "Update Player Rank",
			CustomID: ApplicationCommand.Name,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						Label:       "Rank",
						Style:       discordgo.TextInputShort,
						Placeholder: "Leaderboard position held. Leave empty if unknown",
						CustomID:    modalKeyRank,
						MaxLength:   6,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						Label:       "Rank points",
						Style:       discordgo.TextInputShort,
						Placeholder: "Optional. Prefix ~ if approx, suffix ? if guess",
						CustomID:    modalKeyRankPoints,
						MaxLength:   10,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{Label: "Player",
						Style:       discordgo.TextInputShort,
						Placeholder: "Leave empty if reporting own rank",
						CustomID:    modalKeyPlayer,
						MaxLength:   80,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{Label: "Season",
						Style:       discordgo.TextInputShort,
						Placeholder: "Season number",
						Value:       strconv.Itoa(currentSeason),
						CustomID:    modalKeySeason,
						MaxLength:   2,
					},
				}},
			},
		},
	})
}

func modalDataToDict(cmps []discordgo.MessageComponent) (map[string]string, error) {
	data := make(map[string]string, len(cmps))
	for _, cmp := range cmps {
		row, ok := cmp.(*discordgo.ActionsRow)
		if !ok {
			return nil, fmt.Errorf("unexpected modal data type: (%T) %v", cmp, cmp)
		}
		if len(row.Components) != 1 {
			return nil, fmt.Errorf("unexpected modal row length: %v", len(row.Components))
		}
		input, ok := row.Components[0].(*discordgo.TextInput)
		if !ok {
			return nil, fmt.Errorf("unexpected modal data type in row: (%T) %v", cmp, cmp)
		}
		data[input.CustomID] = input.Value
	}
	return data, nil
}

func parseModalInput(s *discordgo.Session, i *discordgo.InteractionCreate) (ent entry, season int, err error) {
	data, err := modalDataToDict(i.ModalSubmitData().Components)
	if err != nil {
		return ent, 0, err
	}

	if v := data[modalKeyRank]; v != "" {
		u, err := strconv.ParseUint(v, 10, 32)
		if err != nil || u == 0 {
			return ent, 0, errors.New("invalid rank value")
		}
		ent.rank = int(u)
	}

	if v := data[modalKeyRankPoints]; v != "" {
		if l := len(v); l > 0 && v[l-1] == '?' {
			ent.guess = true
			v = v[:l-1]
		}
		if len(v) > 0 && v[0] == '~' {
			ent.approx = true
			v = v[1:]
		}
		u, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return ent, 0, errors.New("invalid rank points value")
		}
		ent.points = int(u)
	}

	if i.Member == nil {
		return ent, 0, errors.New("command should be used within discord guild")
	}
	if v := data[modalKeyPlayer]; v == "" {
		ent.userID = i.Member.User.ID
	} else {
		ent.userID, err = findUserID(s, i.GuildID, v)
		if err != nil {
			log.Println("failed to look up user:", err)
			return ent, 0, errors.New("failed to look up user")
		}
		if ent.userID == "" {
			// don't go crazy with sanitisation, but don't allow callers to pollute with unexpected line breaks
			ent.name = strings.ReplaceAll(v, "\n", " ")
		}
	}

	season = -1
	if v := data[modalKeySeason]; v != "" {
		u, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return ent, 0, errors.New("invalid season value")
		}
		season = int(u)
	}

	if ent.userID != i.Member.User.ID {
		ent.reporterID = i.Member.User.ID
	}
	ent.timestamp = int(time.Now().Unix())

	return ent, season, nil
}

// findUserID returns an empty string and no error if no unambiguous match was found
func findUserID(s *discordgo.Session, guildID string, query string) (string, error) {
	mem, err := s.GuildMembersSearch(guildID, query, 10)
	if err != nil {
		return "", err
	}

	// only use search result if unambiguous or exact match, prioritising username match
	match := ""
	if len(mem) > 1 {
		mem = slices.DeleteFunc(mem, func(m *discordgo.Member) bool {
			if m.User.Username == query {
				match = m.User.ID
				return false
			}
			return m.Nick != query
		})
	}
	if match != "" {
		return match, nil
	}
	if len(mem) == 1 {
		return mem[0].User.ID, nil
	}
	return "", nil
}
