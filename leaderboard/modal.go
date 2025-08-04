package leaderboard

import (
	"fmt"
	"log"
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
						Placeholder: "Leaderboard position held",
						CustomID:    modalKeyRank,
						MinLength:   1,
						MaxLength:   6,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{
						Label:       "Rank points",
						Style:       discordgo.TextInputShort,
						Placeholder: "Optional. prefix ~ if approx, suffix ? if guess",
						CustomID:    modalKeyRankPoints,
						Required:    false,
						MaxLength:   10,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{Label: "Player",
						Style:       discordgo.TextInputShort,
						Placeholder: "Leave empty if reporting own rank",
						CustomID:    modalKeyPlayer,
						Required:    false,
						MaxLength:   80,
					},
				}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.TextInput{Label: "Season",
						Style:       discordgo.TextInputShort,
						Placeholder: "Season number",
						Value:       strconv.Itoa(currentSeason),
						CustomID:    modalKeySeason,
						Required:    false,
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

	u, err := strconv.ParseUint(data[modalKeyRank], 10, 32)
	if err != nil || u == 0 {
		return ent, 0, fmt.Errorf("invalid rank value")
	}
	ent.rank = int(u)

	if v := data[modalKeyRankPoints]; v != "" {
		if l := len(v); l > 0 && v[l-1] == '?' {
			ent.guess = true
			v = v[:l-1]
		}
		if len(v) > 0 && v[0] == '~' {
			ent.approx = true
			v = v[1:]
		}
		u, err = strconv.ParseUint(v, 10, 32)
		if err != nil {
			return ent, 0, fmt.Errorf("invalid rank points value")
		}
		ent.points = int(u)
	}

	if i.Member == nil {
		return ent, 0, fmt.Errorf("command should be used within discord guild")
	}
	if v := data[modalKeyPlayer]; v == "" {
		ent.userID = i.Member.User.ID
	} else {
		mem, err := s.GuildMembersSearch(i.GuildID, v, 2)
		if err != nil {
			log.Println("failed to look up user:", err)
			return ent, 0, fmt.Errorf("failed to look up user")
		}
		// only use search result if unambiguous
		if len(mem) == 1 {
			ent.userID = mem[0].User.ID
		} else {
			// don't go crazy with sanitisation, but don't allow callers to pollute with unexpected line breaks
			ent.name = strings.ReplaceAll(v, "\n", "")
		}
	}

	season = -1
	if v := data[modalKeySeason]; v != "" {
		u, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return ent, 0, fmt.Errorf("invalid season value")
		}
		season = int(u)
	}

	if ent.userID != i.Member.User.ID {
		ent.reporterID = i.Member.User.ID
	}
	ent.timestamp = int(time.Now().Unix())

	return ent, season, nil
}
