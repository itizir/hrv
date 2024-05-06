package main

import (
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

func interactionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
		s.ChannelTyping(i.ChannelID)
		if err := h(s, i); err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: err.Error(),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}
	}
}
