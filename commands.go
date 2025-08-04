package main

import (
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/itizir/hrv/countvotes"
	"github.com/itizir/hrv/leaderboard"
)

type Handler func(s *discordgo.Session, i *discordgo.InteractionCreate) error

var (
	commands = map[*discordgo.ApplicationCommand]Handler{
		countvotes.ApplicationCommand:       countvotes.Handle,
		leaderboard.ApplicationCommand:      leaderboard.Handle,
		leaderboard.ApplicationAdminCommand: leaderboard.HandleAdmin,
	}

	commandHandlers map[string]Handler
)

func init() {
	commandHandlers = make(map[string]Handler)

	for c, h := range commands {
		commandHandlers[c.Name] = h
	}
}

func interactionHandle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var name string
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		name = i.ApplicationCommandData().Name
	case discordgo.InteractionMessageComponent:
		spl := strings.SplitN(i.MessageComponentData().CustomID, ":", 2)
		name = spl[0]
	case discordgo.InteractionModalSubmit:
		name = i.ModalSubmitData().CustomID
	}

	if h, ok := commandHandlers[name]; ok {
		if err := h(s, i); err != nil {
			log.Println("handler failed:", err)
		}
	} else {
		log.Println("no handler for", name, i)
	}
}
