package leaderboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

type entry struct {
	userID     string
	name       string
	rank       int
	points     int
	approx     bool
	guess      bool
	reporterID string
	timestamp  int
}

func userMention(id string) string {
	return (&discordgo.User{ID: id}).Mention()
}

func (r entry) String() string {
	p := message.NewPrinter(language.English)

	player := r.name
	if r.userID != "" {
		player = userMention(r.userID)
	}
	pts := "(?)"
	if r.points > 0 {
		rounded := int(math.Round(float64(r.points) / 1000.))
		if r.guess {
			pts = p.Sprintf("(~%dk?)", number.Decimal(rounded))
		} else if r.approx {
			pts = p.Sprintf("(~%dk)", number.Decimal(rounded))
		} else {
			pts = p.Sprintf("(%d)", number.Decimal(r.points))
		}
	}
	reporter := ""
	if r.reporterID != "" {
		reporter = " by " + userMention(r.reporterID)
	}

	return fmt.Sprintf("%d\\. %s %s — <t:%d:R>%s", r.rank, player, pts, r.timestamp, reporter)
}

func getRankAndName(raw string) (rank int, nameOrID string, isID bool) {
	sep := "\\. "
	i := strings.Index(raw, "\\. ")
	if i >= 0 {
		rank, _ = strconv.Atoi(raw[:i])
		raw = raw[i+len(sep):]
	}

	i = strings.LastIndex(raw, " (")
	if i >= 0 {
		raw = raw[:i]
	}

	nameOrID = raw
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "<@"), ">")
	if _, err := strconv.Atoi(raw); err == nil {
		nameOrID = raw
		isID = true
	}

	return rank, nameOrID, isID
}
