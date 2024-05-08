package main

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/itizir/hrv/countvotes"
)

type Handler func(s *discordgo.Session, i *discordgo.InteractionCreate) error

var (
	commands = map[*discordgo.ApplicationCommand]Handler{
		countvotes.ApplicationCommand: countvotes.Handle,
	}

	commandHandlers map[string]Handler
)

func init() {
	commandHandlers = make(map[string]Handler)

	for c, h := range commands {
		commandHandlers[c.Name] = h
	}
}

func interactionHandler(withAck bool) func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			if withAck {
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				})
				if err != nil {
					log.Println("failed to respond to interaction:", err)
					return
				}
			}
			if err := h(s, i); err != nil {
				log.Println("handler failed:", err)
			}
		}
	}
}
