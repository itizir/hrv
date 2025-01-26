package countvotes

import (
	"slices"
	"strings"
)

type contest struct {
	posts []*post
}

// Prioritise breaking ambiguities by first considering main vote, then by overall least- to most-given votes.
// This basically applies if a given submission is an undisputed winner of more than one category.
func (con contest) orderedCategories() []string {
	counts := make(map[string]int)
	for _, p := range con.posts {
		for k, v := range p.reactions {
			counts[k] += len(v)
		}
	}

	categories := append([]string{emojiMain}, emojiSecondary...)
	slices.SortStableFunc(categories[1:], func(a, b string) int {
		return counts[a] - counts[b]
	})

	return categories
}

// Pick winners by prioritising giving the win to submissions that have the top number of votes in a given category:
// if two entries are tied, but one of them happens to be an undisputed winner in another category, break the tie by
// giving the win of the tied category to the submission not eligible for another win.
// If this is still not enough, attempt to break ties by including number of plays and other votes,
// as described in `postCmp`.
func (con contest) winners() []string {
	if len(con.posts) == 0 {
		return nil
	}

	win := make(map[string]*post)
	categories := con.orderedCategories()
	superficialTies := true
	mainCategoryMaxVotes := 0

	for {
		foundNewWinner := false
		for i, cat := range categories {
			if _, ok := win[cat]; ok {
				continue
			}
			candidates := slices.Clone(con.posts)
			candidates = slices.DeleteFunc(candidates, func(c *post) bool {
				return c.won != "" || c.numReact(cat) == 0
			})
			if len(candidates) == 0 {
				continue
			}
			slices.SortStableFunc(candidates, postCmp(cat))

			numTied := 0
			maxVotes := candidates[0].numReact(cat)
			if i == 0 && mainCategoryMaxVotes == 0 {
				mainCategoryMaxVotes = maxVotes
			}
			for _, c := range candidates[1:] {
				// always consider tie-breakers for main category!
				if (i > 0 && superficialTies && c.numReact(cat) == maxVotes) || postCmp(cat)(candidates[0], c) == 0 {
					numTied++
				} else {
					break
				}
			}

			if numTied == 0 {
				candidates[0].won = cat
				win[cat] = candidates[0]
				foundNewWinner = true
			}
		}

		if !foundNewWinner {
			if superficialTies {
				superficialTies = false
			} else {
				break
			}
		}
		if len(win) == len(categories) {
			break
		}
	}

	var retval []string
	for _, cat := range append([]string{emojiMain}, emojiSecondary...) {
		if w, ok := win[cat]; ok {
			res := w.String()
			if cat == emojiMain && w.numReact(emojiMain) < mainCategoryMaxVotes {
				res = "COULD NOT BREAK TIE! OVERALL WINNER HAS FEWER POINTS THAN OTHER SUBMISSIONS!\n" + res
			}

			var better, ties []string
			for _, p := range con.posts {
				if p.thread == w.thread {
					continue
				}
				if np, nw := p.numReact(cat), w.numReact(cat); np > nw {
					if p.won == "" {
						res += "... but shouldn't " + p.thread + " have won?!?"
					} else {
						better = append(better, p.thread)
					}
				} else if np == nw {
					ties = append(ties, p.thread)
				}
			}
			if len(better) > 0 || len(ties) > 0 {
				if len(better) > 0 {
					res += "\n   more votes, but won something else:\n    " + strings.Join(better, "\n    ")
				}
				if len(ties) > 0 {
					res += "\n   tied number of votes:\n    " + strings.Join(ties, "\n    ")
				}
			}

			retval = append(retval, res)
		}
	}
	return retval
}
