package leaderboard

import (
	"errors"
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

// hmm, should we do partial matching, or somehow guild-specific? for now all a bit moot anyway...
var leaderboardsForumName = "│leaderboards"

const threadNamePrefix = "Season "

func initialMessage(appID string) string {
	// OK, hardcoded MYM joke right there... But also probably accurate starting point.
	msg := `1\. Fnovc2d (???)

Add master rank players to the leaderboard by calling ` + userMention(appID) + `'s ` + "`rank`" + ` command (that can be invoked by simply typing ` + "`/rank`" + ` from anywhere on this server).
You may optionally report exact (e.g. ` + "`35123`" + `), approximate (e.g. ` + "`~77000`" + `), or guessed (e.g. ` + "`180000?`" + `) rank points.
If reporting for another Discord member, it isn't necessary to enter their whole name or username as long as it unambiguously identifies them; priority will be given to exact _username_ match.`
	return msg
}

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
		return nil, 0, errors.New("failed to fetch active threads")
	}
	if thr := selectThread(thrs.Threads); thr != nil {
		return thr, season, nil
	}

	for {
		thrs, err := s.ThreadsArchived(c.ID, nil, 0)
		if err != nil {
			log.Printf("failed to fetch archived threads in %v: %v", c.ID, err)
			return nil, 0, errors.New("failed to fetch archived threads")
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
		return nil, 0, errors.New("failed to find any active leaderboards")
	}

	return latest, latestSeason, nil
}

func getLeaderboardsForum(s *discordgo.Session, guildID string) (*discordgo.Channel, error) {
	chans, err := s.GuildChannels(guildID)
	if err != nil {
		log.Printf("failed to enumerate guild %v channels: %v", guildID, err)
		return nil, errors.New("failed to enumerate guild channels")
	}

	for _, c := range chans {
		if c.Type == discordgo.ChannelTypeGuildForum && c.Name == leaderboardsForumName {
			return c, nil
		}
	}

	return nil, errors.New("could not find the leaderboards forum")
}

func createSeasonThread(s *discordgo.Session, guildID string, appID string, name string) error {
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
		return errors.New("season number should not be negative")
	}

	c, err := getLeaderboardsForum(s, guildID)
	if err != nil {
		return err
	}

	thr, err := s.ForumThreadStart(c.ID, name, 0, initialMessage(appID))
	if err != nil {
		return err
	}

	s.ChannelMessagePin(thr.ID, thr.ID) // try to pin, if permissions allow...
	return nil
}
