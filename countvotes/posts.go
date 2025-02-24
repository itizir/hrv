package countvotes

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

type post struct {
	thread    string
	author    string
	reactions map[string][]string
	won       string
}

// If tied in number of votes in that category, try to break the tie by considering
// first ratio to number of plays, then consider total votes across all categories,
// and finally votes in the overall category.
func postCmp(e string) func(p, q *post) int {
	return func(p, q *post) int {
		if pc, qc := p.numReact(e), q.numReact(e); pc != qc {
			return qc - pc
		}
		pn, qn := p.numReact(emojiPlayed), q.numReact(emojiPlayed)
		if pn != qn {
			return pn - qn
		}
		if pc, qc := qn*p.totVotes(), pn*q.totVotes(); pc != qc {
			return qc - pc
		}
		if pc, qc := qn*p.numReact(emojiMain), pn*q.numReact(emojiMain); pc != qc {
			return qc - pc
		}
		return 0
	}
}

func (p post) String() string {
	if p.thread == "" {
		return "Empty post"
	}

	var str string

	if p.won == emojiMain {
		str += "Best overall! "
	} else if p.won != "" {
		str += fmt.Sprintf("Most %s: ", p.won)
	}

	author := "_unknown_"
	if p.author != "" {
		author = userMention(p.author)
	}
	str += fmt.Sprintf("%s (%s)", p.thread, author)

	if p.won == "" {
		return str
	}

	name := "votes"
	n := p.numReact(p.won)
	// Probably always true, but just to be safe...
	if e := knownEmojis[p.won]; e != nil {
		name = e.MessageFormat()
	} else if n == 1 {
		name = "vote"
	}
	str += fmt.Sprintf(" — with %v %s", n, name)

	return str
}

func (p post) numReact(e string) int {
	return len(p.reactions[e])
}

func (p post) totVotes() int {
	t := 0
	for e, u := range p.reactions {
		if e == emojiPlayed {
			continue
		}
		t += len(u)
	}
	return t
}

func fetchPosts(s *discordgo.Session, guildID, chanID string, excludedVoters, excludedContestants map[string]bool) ([]*post, error) {
	var posts []*post

	t0 := time.Now()
	defer func() {
		log.Printf("fetching %v posts took %v", len(posts), time.Since(t0))
	}()

	active, err := s.GuildThreadsActive(guildID)
	if err != nil {
		return nil, err
	}
	threads := slices.DeleteFunc(active.Threads, func(c *discordgo.Channel) bool {
		return c.ParentID != chanID
	})
	archived, err := s.ThreadsArchived(chanID, nil, 0)
	if err != nil {
		return nil, err
	}
	threads = append(threads, archived.Threads...)

	threads = slices.DeleteFunc(threads, func(c *discordgo.Channel) bool {
		return excludedContestants[c.OwnerID]
	})

	postsChan := make(chan *post, len(threads))
	errChan := make(chan error, len(threads))
	for _, thread := range threads {
		go func(thread *discordgo.Channel) {
			msg, err := s.ChannelMessage(thread.ID, thread.ID)
			if err != nil {
				errChan <- err
				return
			}

			rcts := make(map[string][]string)
			for _, react := range msg.Reactions {
				if _, ok := knownEmojis[react.Emoji.Name]; !ok {
					continue
				}
				knownEmojis[react.Emoji.Name] = react.Emoji

				// If the number of voters gets over 100... would need to scroll through pages.
				// We're far needing this at the moment, though.
				users, err := s.MessageReactions(thread.ID, thread.ID, react.Emoji.APIName(), 100, "", "")
				if err != nil {
					errChan <- err
					return
				}

				var userStrings []string
				for _, u := range users {
					if excludedVoters[u.ID] {
						continue
					}
					userStrings = append(userStrings, u.ID)
				}
				rcts[react.Emoji.Name] = userStrings
			}

			postsChan <- &post{
				thread:    thread.Mention(),
				author:    msg.Author.ID,
				reactions: rcts,
			}
		}(thread)
	}

	for range threads {
		select {
		case p := <-postsChan:
			posts = append(posts, p)
		case err := <-errChan:
			return nil, err
		}
	}

	if len(posts) == 0 {
		return nil, errors.New("did not find any posts in the thread")
	}

	return posts, nil
}

func userMention(id string) string {
	return (&discordgo.User{ID: id}).Mention()
}
