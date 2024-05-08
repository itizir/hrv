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
func (con contest) validate() []string {
	type stats struct {
		submissions    int
		votesTotal     int
		mainVotesTotal int
		selfVote       bool
		overVoted      bool
		missingPlayed  bool
	}

	participants := make(map[string]*stats)

	for _, p := range con.posts {
		s, ok := participants[p.author]
		if !ok {
			s = &stats{}
			participants[p.author] = s
		}
		s.submissions++

		hasPlayed := make(map[string]bool)
		for _, u := range p.reactions[emojiPlayed] {
			hasPlayed[u] = true
		}

		numVotesPost := make(map[string]int)
		for k, voters := range p.reactions {
			if k == emojiPlayed {
				continue
			}
			for _, voter := range voters {
				s, ok := participants[voter]
				if !ok {
					s = &stats{}
					participants[voter] = s
				}

				if voter == p.author {
					s.selfVote = true
				}
				if !hasPlayed[voter] {
					s.missingPlayed = true
				}

				if k == emojiMain {
					s.mainVotesTotal++
				} else {
					s.votesTotal++
					numVotesPost[voter]++
					if numVotesPost[voter] > 2 {
						s.overVoted = true
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
		var offenses []string
		if s.submissions > 1 {
			offenses = append(offenses, "made more than one submission")
		}
		if s.selfVote {
			offenses = append(offenses, "voted for their own submission")
		}
		if s.mainVotesTotal > 1 {
			offenses = append(offenses, "gave out too many "+mainVote)
		}
		if s.votesTotal > len(con.posts) {
			offenses = append(offenses, "gave out too many votes overall")
		}
		if s.overVoted {
			offenses = append(offenses, "gave out too many votes to a given submission")
		}
		if s.missingPlayed {
			offenses = append(offenses, "voted without reacting with "+playedReaction)
		}

		if l := len(offenses); l == 0 {
			continue
		} else if l > 1 {
			offenses[l-1] = "_and_ " + offenses[l-1]
		}
		irregularities = append(irregularities, fmt.Sprintf("%s is on the naughty list! They %s! 🙀", p, strings.Join(offenses, ", ")))
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
