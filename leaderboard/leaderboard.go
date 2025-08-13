package leaderboard

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	unorderedPrefix = "- "

	discordMessageLimit = 2000
)

// returns the ID of the leaderboard channel
func editLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate) (string, error) {
	ent, season, err := parseModalInput(s, i)
	if err != nil {
		return "", err
	}

	thread, _, err := getSeasonThread(s, i.GuildID, i.AppID, season)
	if err != nil {
		return "", err
	}

	ld, err := getLeaderboardData(s, thread)
	if err != nil {
		log.Printf("failed to fetch leaderboard data from channel %v: %v", thread.ID, err)
		return "", errors.New("failed to fetch messages")
	}

	if err := ld.addEntry(ent); err != nil {
		return "", err
	}

	if err := ld.updateMessages(s); err != nil {
		log.Printf("failed to edit leaderboard %v: %v", thread.ID, err)
		return "", errors.New("failed to edit leaderboard message")
	}

	return thread.ID, nil
}

type discordMessage struct {
	id      string
	content string
}

type leaderboardData struct {
	ranked    []string
	unordered []string

	channelID string
	msgs      []discordMessage

	appID string
}

func getLeaderboardData(s *discordgo.Session, thread *discordgo.Channel) (leaderboardData, error) {
	ld := leaderboardData{channelID: thread.ID, appID: thread.OwnerID}

	// ids are basically timestamps, and 'after' is strict, so decrement initial message by one...
	tID, err := strconv.ParseUint(thread.ID, 10, 64)
	if err != nil {
		return ld, err
	}
	afterID := strconv.FormatUint(tID-1, 10)

	// placeholders + top message + instructions. could fetch more to be sure, since we filter out other messages for safety below.
	msgs, err := s.ChannelMessages(thread.ID, numPlaceholderMessages+2, "", afterID, "")
	if err != nil {
		return ld, err
	}
	if len(msgs) == 0 {
		return ld, errors.New("no messages in channel")
	}
	msgs = slices.DeleteFunc(msgs, func(msg *discordgo.Message) bool {
		return msg.Type != discordgo.MessageTypeDefault || msg.Author.ID != thread.OwnerID
	})
	// we get them in anti-chronological order
	slices.Reverse(msgs)

	inUnorderedSection := false
	for _, msg := range msgs {
		ld.msgs = append(ld.msgs, discordMessage{id: msg.ID, content: msg.Content})
	linesLoop:
		for _, l := range strings.Split(msg.Content, "\n") {
			switch l {
			case "", placeholderMessage, leaderboardMessagePrefix:
				continue
			case unknownRankMessagePrefix:
				inUnorderedSection = true
				continue
			case instructionsMessagePrefix:
				break linesLoop
			}

			if inUnorderedSection {
				ld.unordered = append(ld.unordered, l)
			} else {
				ld.ranked = append(ld.ranked, l)
			}
		}
	}

	return ld, nil
}

func (ld leaderboardData) updateMessages(s *discordgo.Session) error {
	currentMessageIndex := 0
	currentPageContent := leaderboardMessagePrefix + "\n"

	postPage := func() error {
		if currentMessageIndex >= len(ld.msgs) {
			return fmt.Errorf("ran out of pages (%d available)", len(ld.msgs))
		}
		msg := ld.msgs[currentMessageIndex]
		if currentPageContent != msg.content {
			if _, err := s.ChannelMessageEdit(ld.channelID, msg.id, currentPageContent); err != nil {
				return err
			}
		}
		currentMessageIndex++
		currentPageContent = ""
		return nil
	}

	printLines := func(lines []string) error {
		for _, l := range lines {
			// +1 to account for the newline separator
			if len(currentPageContent)+len(l)+1 > discordMessageLimit {
				if err := postPage(); err != nil {
					return err
				}
			} else {
				// avoid newline if it's the start of a page
				currentPageContent += "\n"
			}
			currentPageContent += l
		}
		// could also compact the different sections onto the same pages if space allows, but ehh.
		// this is a little cleaner, and if we need more space we can reserve more messages.
		if currentPageContent != "" {
			return postPage()
		}
		return nil
	}

	if err := printLines(ld.ranked); err != nil {
		return err
	}

	if len(ld.unordered) > 0 {
		currentPageContent = unknownRankMessagePrefix + "\n"

		// ironic... :P
		slices.Sort(ld.unordered)
		if err := printLines(ld.unordered); err != nil {
			return err
		}
	}

	currentPageContent = instructionsMessage(ld.appID)
	if err := postPage(); err != nil {
		return err
	}

	for currentMessageIndex < len(ld.msgs) {
		currentPageContent = placeholderMessage
		if err := postPage(); err != nil {
			return err
		}
	}

	return nil
}

func (ld *leaderboardData) addEntry(ent entry) error {
	rankUnknown := ent.rank == 0

	var (
		added  bool
		update []string
	)
	for _, e := range ld.ranked {
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
	ld.ranked = update

	ent.rank = 0
	newEntry := unorderedPrefix + ent.String()
	if added {
		ld.unordered = slices.DeleteFunc(ld.unordered, func(e string) bool { return e == newEntry })
	} else {
		if slices.Contains(ld.unordered, newEntry) {
			return errors.New("player already reported as master")
		}
		ld.unordered = append(ld.unordered, newEntry)
	}
	return nil
}

func (ld *leaderboardData) removeEntries(nameOrMention string, rank int) {
	if rank == 0 {
		ld.ranked = slices.DeleteFunc(ld.ranked, func(e string) bool {
			_, after, _ := strings.Cut(e, "\\. ")
			return strings.HasPrefix(after, nameOrMention)
		})

		prefix := unorderedPrefix + nameOrMention
		ld.unordered = slices.DeleteFunc(ld.unordered, func(e string) bool {
			return strings.HasPrefix(e, prefix)
		})
	}

	if rank > 0 {
		prefix := fmt.Sprintf("%d\\. %s", rank, nameOrMention)
		ld.ranked = slices.DeleteFunc(ld.ranked, func(e string) bool {
			return strings.HasPrefix(e, prefix)
		})
	}
}
