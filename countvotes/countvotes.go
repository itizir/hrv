package countvotes

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

const (
	optionChannel = "channel"

	selectExcludeVoters      = "exclude_voters"
	selectExcludeContestants = "exclude_contestants"

	buttonValidate = "validate"
	buttonResults  = "results"
	buttonCancel   = "cancel"

	maxSelections = 25 // max allowed in selectors... eh, hopefully enough for our purpose here.
)

var (
	ApplicationCommand = &discordgo.ApplicationCommand{
		Name:        "countvotes",
		Description: "Count votes and determine contest winners",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionChannel,
				Name:         optionChannel,
				Description:  "Forum channel of the contest",
				Required:     true,
				Autocomplete: true,
				ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildForum},
			},
		},
	}
)

type options struct {
	channel             string
	validateOnly        bool
	excludedVoters      []string
	excludedContestants []string
}

func Handle(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if i.Type == discordgo.InteractionApplicationCommand {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Fetching list of participants, and then waiting for caller input. Be patient!"},
		})
		if err != nil {
			return err
		}

		opts := commandOptions(i.ApplicationCommandData())
		unknown, err := fetchUnknownUsers(s, i.GuildID, opts.channel)
		if err != nil {
			c := fmt.Sprintf("Failed to fetch data: %v.", err)
			_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &c})
			return err
		}
		if len(unknown) > maxSelections {
			unknown = unknown[:maxSelections]
		}
		opts.excludedVoters = append(opts.excludedVoters, unknown...)
		opts.excludedContestants = append(opts.excludedContestants, unknown...)

		_, err = s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
			Components: components(opts),
			Flags:      discordgo.MessageFlagsEphemeral,
		})
		return err
	} else if i.Type != discordgo.InteractionMessageComponent {
		return fmt.Errorf("unexpected interaction type: %v", i.Type)
	}

	msg := i.Message
	if msg == nil {
		return errors.New("interaction message nil")
	}

	opts := fromDefaultValues(msg.Components)
	action := messageInteractionUpdate(i.MessageComponentData(), &opts)

	switch action {
	case selectExcludeVoters, selectExcludeContestants:
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: components(opts),
			},
		})
	case buttonValidate, buttonResults, buttonCancel:
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate})
		if err != nil {
			return err
		}
		if msg.MessageReference == nil {
			return errors.New("nil message reference")
		}
	case "":
		return errors.New("no action")
	default:
		return fmt.Errorf("undefined action %q", action)
	}

	if action == buttonCancel {
		err := s.ChannelMessageDelete(msg.MessageReference.ChannelID, msg.MessageReference.MessageID)
		if err != nil {
			return err
		}
	} else {
		content := "Fetching data and determining results..."
		_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content:    &content,
			Components: &([]discordgo.MessageComponent{}),
		})
		if err != nil {
			return err
		}
		resp := determineResults(s, i.GuildID, opts)
		_, err = s.ChannelMessageEdit(msg.MessageReference.ChannelID, msg.MessageReference.MessageID, resp)
		if err != nil {
			return err
		}
	}

	return s.InteractionResponseDelete(i.Interaction)
}

func determineResults(s *discordgo.Session, guildID string, opts options) string {
	resp := ""

	excludedVoters := make(map[string]bool)
	if len(opts.excludedVoters) > 0 {
		resp += "Ignored voters: "
	}
	for i, u := range opts.excludedVoters {
		resp += userMention(u)
		if i < len(opts.excludedVoters)-1 {
			resp += ", "
		} else {
			resp += ".\n"
		}
		excludedVoters[u] = true
	}

	excludedContestants := make(map[string]bool)
	if len(opts.excludedContestants) > 0 {
		resp += "Ignored contestants: "
	}
	for i, u := range opts.excludedContestants {
		resp += userMention(u)
		if i < len(opts.excludedContestants)-1 {
			resp += ", "
		} else {
			resp += ".\n"
		}
		excludedContestants[u] = true
	}

	if len(resp) > 0 {
		resp += "\n"
	}

	posts, err := fetchPosts(s, guildID, opts.channel, excludedVoters, excludedContestants)
	if err != nil {
		return fmt.Sprintf("%sOops! Failed to get the data from <#%v>: %v.", resp, opts.channel, err)
	}

	con := contest{posts: posts}

	hasIrregularities := false
	if irregularities := con.validate(excludedVoters); len(irregularities) > 0 {
		hasIrregularities = true
		resp += "Oh no! Found some irregularities:\n"
		resp += "- " + strings.Join(irregularities, "\n- ") + "\n"
		resp += "...so the results shouldn't be trusted. 😿\n\n"
	}

	win := con.winners()
	if l := len(win); l == 0 {
		resp += fmt.Sprintf("Ooof! Could not determine _any_ winners in <#%v>!", opts.channel)
		return resp
	} else if l < 1+len(emojiSecondary) {
		hasIrregularities = true
		resp += "Could not determine winners for all categories: undecidable ties, or too few eligible submissions. Sad. Well, anyway...\n\n"
	}

	if opts.validateOnly {
		if hasIrregularities {
			resp = fmt.Sprintf("Validating <#%s> without revealing results...\n\n%s", opts.channel, resp)
		} else {
			resp = fmt.Sprintf("No irregularities in <#%s>! 👏\n\n%s", opts.channel, resp)
		}
	} else {
		resp += fmt.Sprintf("🥁 Without further ado, the winners of <#%s>:\n", opts.channel)
		resp += "- " + strings.Join(win, "\n- ") + "\n"
		resp += "Congratulations! 🎉"
	}

	return resp
}

func commandOptions(data discordgo.ApplicationCommandInteractionData) options {
	var opts options
	for _, o := range data.Options {
		switch o.Name {
		case optionChannel:
			opts.channel, _ = o.Value.(string)
		}
	}
	return opts
}

func messageInteractionUpdate(data discordgo.MessageComponentInteractionData, opts *options) string {
	spl := strings.Split(data.CustomID, ":")
	if len(spl) < 3 {
		return ""
	}
	opts.channel = spl[1]
	action := spl[2]
	switch action {
	case selectExcludeVoters:
		opts.excludedVoters = data.Values
	case selectExcludeContestants:
		opts.excludedContestants = data.Values
	case buttonValidate:
		opts.validateOnly = true
	}
	return action
}

func components(opts options) []discordgo.MessageComponent {
	zero := 0

	elementID := func(id string) string {
		return ApplicationCommand.Name + ":" + opts.channel + ":" + id
	}

	var excludedVoters, excludedContestants []discordgo.SelectMenuDefaultValue
	for _, u := range opts.excludedVoters {
		excludedVoters = append(excludedVoters, discordgo.SelectMenuDefaultValue{ID: u, Type: discordgo.SelectMenuDefaultValueUser})
	}
	for _, u := range opts.excludedContestants {
		excludedContestants = append(excludedContestants, discordgo.SelectMenuDefaultValue{ID: u, Type: discordgo.SelectMenuDefaultValueUser})
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:      discordgo.UserSelectMenu,
					CustomID:      elementID(selectExcludeVoters),
					MinValues:     &zero,
					MaxValues:     maxSelections,
					Placeholder:   "Excluded Voters",
					DefaultValues: excludedVoters,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:      discordgo.UserSelectMenu,
					CustomID:      elementID(selectExcludeContestants),
					MinValues:     &zero,
					MaxValues:     maxSelections,
					Placeholder:   "Excluded Contestants",
					DefaultValues: excludedContestants,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Validate Only",
					Style:    discordgo.SuccessButton,
					CustomID: elementID(buttonValidate),
				},
				discordgo.Button{
					Label:    "Get Results",
					Style:    discordgo.PrimaryButton,
					CustomID: elementID(buttonResults),
				},
				discordgo.Button{
					Label:    "Cancel",
					Style:    discordgo.DangerButton,
					CustomID: elementID(buttonCancel),
				},
			},
		},
	}
}

func fromDefaultValues(c []discordgo.MessageComponent) options {
	opts := options{}
	if len(c) < 2 {
		return opts
	}
	getPlaceholderValues := func(c []discordgo.MessageComponent) []string {
		if len(c) == 0 {
			return nil
		}
		s, ok := c[0].(*discordgo.SelectMenu)
		if !ok {
			return nil
		}
		var vals []string
		for _, v := range s.DefaultValues {
			vals = append(vals, v.ID)
		}
		return vals
	}
	opts.excludedVoters = getPlaceholderValues(c[0].(*discordgo.ActionsRow).Components)
	opts.excludedContestants = getPlaceholderValues(c[1].(*discordgo.ActionsRow).Components)
	return opts
}

func fetchUnknownUsers(s *discordgo.Session, guildID, channelID string) ([]string, error) {
	p, err := fetchPosts(s, guildID, channelID, nil, nil)
	if err != nil {
		return nil, err
	}

	uniqueUsers := make(map[string]struct{})
	for _, p := range p {
		uniqueUsers[p.author] = struct{}{}
		for _, r := range p.reactions {
			for _, u := range r {
				uniqueUsers[u] = struct{}{}
			}
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(uniqueUsers))
	uc := make(chan string, len(uniqueUsers))
	ec := make(chan error, len(uniqueUsers))
	for u := range uniqueUsers {
		go func(u string) {
			defer wg.Done()
			if _, err := s.GuildMember(guildID, u); err != nil {
				if e, ok := err.(*discordgo.RESTError); ok && e.Message.Code == discordgo.ErrCodeUnknownMember {
					uc <- u
				} else {
					ec <- err
				}
			}
		}(u)
	}
	wg.Wait()
	close(uc)
	close(ec)

	if err := <-ec; err != nil {
		return nil, err
	}

	var users []string
	for u := range uc {
		users = append(users, u)
	}
	slices.Sort(users)
	return users, nil
}
