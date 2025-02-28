package countvotes

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Checks for irregularities:
// - no more than one submission per participant
// - no voting on one's own submission
// - only a single 'main' vote allowed per voter
// - only as many 'secondary' votes allowed as number of contest entries
// - max 2 'secondary' votes per submission per voter
// - voters should mark submissions they have evaluated with the 'played' reaction
func (con contest) validate(excludedVoters map[string]bool) []string {
	type stats struct {
		submissions    int
		votesTotal     int
		mainVotesTotal int
		playedTotal    int
		selfVote       bool
		overVoted      []string
		missingPlayed  []string
	}

	participants := make(map[string]*stats)
	getStats := func(p string) *stats {
		s, ok := participants[p]
		if !ok {
			s = &stats{}
			participants[p] = s
		}
		return s
	}

	for _, p := range con.posts {
		getStats(p.author).submissions++

		hasPlayed := make(map[string]bool)
		for _, u := range p.reactions[emojiPlayed] {
			hasPlayed[u] = true
			getStats(u).playedTotal++
		}

		numVotesPost := make(map[string]int)
		for k, voters := range p.reactions {
			if k == emojiPlayed {
				continue
			}
			for _, voter := range voters {
				s := getStats(voter)

				if voter == p.author {
					s.selfVote = true
				}
				if l := len(s.missingPlayed); !hasPlayed[voter] && (l == 0 || s.missingPlayed[l-1] != p.thread) {
					s.missingPlayed = append(s.missingPlayed, p.thread)
				}

				if k == emojiMain {
					s.mainVotesTotal++
				} else {
					s.votesTotal++
					numVotesPost[voter]++
					if numVotesPost[voter] == 3 { // max 2 per submission
						s.overVoted = append(s.overVoted, p.thread)
					}
				}
			}
		}
	}

	mainVote := emojiMain
	if e := knownEmojis[emojiMain]; e != nil {
		mainVote = e.MessageFormat()
	}

	playedReaction := emojiPlayed
	if e := knownEmojis[emojiPlayed]; e != nil {
		playedReaction = e.MessageFormat()
	}

	var irregularities []string
	for p, s := range participants {
		if excludedVoters[p] {
			continue
		}

		var offenses []string
		if s.submissions > 1 {
			offenses = append(offenses, "made more than one submission")
		}
		if s.selfVote {
			offenses = append(offenses, "voted for their own submission")
		}
		if s.mainVotesTotal > 1 {
			offenses = append(offenses, fmt.Sprintf("gave out %d %s", s.mainVotesTotal, mainVote))
		}
		if s.votesTotal > len(con.posts) {
			offenses = append(offenses, fmt.Sprintf("gave out %d instead of max %d votes overall", s.votesTotal, len(con.posts)))
		}
		if len(s.overVoted) > 0 {
			offenses = append(offenses, fmt.Sprintf("gave out too many votes to %s", strings.Join(s.overVoted, ", ")))
		}
		if len(s.missingPlayed) > 0 {
			offenses = append(offenses, fmt.Sprintf("voted without reacting with %s on %s", playedReaction, strings.Join(s.missingPlayed, ", ")))
		}
		if s.submissions > 0 && s.mainVotesTotal+s.votesTotal+s.playedTotal == 0 {
			offenses = append(offenses, "seem to not have made any efforts in voting despite making a contest submission")
		}

		if l := len(offenses); l == 0 {
			continue
		} else if l > 1 {
			offenses[l-1] = "_and_ " + offenses[l-1]
		}
		irregularities = append(irregularities, fmt.Sprintf("%s is on the naughty list! They %s! 🙀", userMention(p), strings.Join(offenses, ", ")))
	}

	// Make the ordering deterministic, but just to an approximate thing for biggest offenders first...
	slices.SortFunc(irregularities, func(a, b string) int {
		if len(a) == len(b) {
			return cmp.Compare(a, b)
		}
		return len(b) - len(a)
	})

	return irregularities
}
