package leaderboard

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	if env := os.Getenv("LEADERBOARDS_FORUM_NAME"); env != "" {
		leaderboardsForumName = env
	}
}

var leaderboardsForumName = "leaderboards"

const (
	threadNamePrefix = "Season "

	// OK, hardcoded MYM joke right there... But also probably accurate starting point.
	initialMessage = "1\\. Fnovc2d (???)"
)

func getSeasonThread(s *discordgo.Session, guildID, authorID string, season int) (*discordgo.Channel, int, error) {
	latestSeason := -1
	var latest *discordgo.Channel
	selectThread := func(thrs []*discordgo.Channel) *discordgo.Channel {
		for _, thr := range thrs {
			if thr.ThreadMetadata.Locked || thr.OwnerID != authorID {
				continue
			}
			name := strings.TrimPrefix(thr.Name, threadNamePrefix)
			name = strings.Split(name, " ")[0]
			i, err := strconv.Atoi(name)
			if err != nil {
				continue
			}
			if i == season {
				return thr
			}
			if i > latestSeason {
				latestSeason = i
				latest = thr
			}
		}
		return nil
	}

	c, err := getLeaderboardsForum(s, guildID)
	if err != nil {
		return nil, 0, err
	}

	thrs, err := s.ThreadsActive(c.ID)
	if err != nil {
		log.Printf("failed to fetch active threads in %v: %v", c.ID, err)
		return nil, 0, fmt.Errorf("failed to fetch active threads")
	}
	if thr := selectThread(thrs.Threads); thr != nil {
		return thr, season, nil
	}

	for {
		thrs, err := s.ThreadsArchived(c.ID, nil, 0)
		if err != nil {
			log.Printf("failed to fetch archived threads in %v: %v", c.ID, err)
			return nil, 0, fmt.Errorf("failed to fetch archived threads")
		}
		if thr := selectThread(thrs.Threads); thr != nil {
			return thr, season, nil
		}
		if !thrs.HasMore {
			break
		}
	}

	if season >= 0 && season != latestSeason {
		return nil, 0, fmt.Errorf("season %d not tracked", season)
	}
	if latest == nil {
		return nil, 0, fmt.Errorf("failed to find any active leaderboards")
	}

	return latest, latestSeason, nil
}

func getLeaderboardsForum(s *discordgo.Session, guildID string) (*discordgo.Channel, error) {
	chans, err := s.GuildChannels(guildID)
	if err != nil {
		log.Printf("failed to enumerate guild %v channels: %v", guildID, err)
		return nil, fmt.Errorf("failed to enumerate guild channels")
	}

	for _, c := range chans {
		if c.Type == discordgo.ChannelTypeGuildForum && c.Name == leaderboardsForumName {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not find the leaderboards forum")
}

func createSeasonThread(s *discordgo.Session, guildID string, name string) error {
	if !strings.HasPrefix(name, threadNamePrefix) {
		return fmt.Errorf("invalid season name (should start with %q)", threadNamePrefix)
	}

	str := strings.TrimPrefix(name, threadNamePrefix)
	str = strings.Split(str, " ")[0]
	i, err := strconv.Atoi(str)
	if err != nil {
		return fmt.Errorf("invalid season number: %w", err)
	}
	if i < 0 {
		return fmt.Errorf("season number should not be negative")
	}

	c, err := getLeaderboardsForum(s, guildID)
	if err != nil {
		return err
	}

	_, err = s.ForumThreadStart(c.ID, name, 0, initialMessage)
	return err
}
