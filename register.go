package main

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func registerCommands(s *discordgo.Session) error {
	log.Println("Registering commands...")

	for c := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", c)
		if err != nil {
			return err
		}
		guild := " globally"
		if cmd.GuildID != "" {
			guild = " to " + cmd.GuildID
		}
		log.Printf("Added %s (%s)%s", cmd.Name, cmd.ID, guild)
	}

	log.Println("Done registering commands!")
	return nil
}

func cleanupCommands(s *discordgo.Session) error {
	log.Println("Cleaning up commands...")

	guilds, err := s.UserGuilds(200, "", "", false)
	if err != nil {
		return err
	}
	guildIDs := []string{""}
	for _, g := range guilds {
		guildIDs = append(guildIDs, g.ID)
		log.Println("Belongs to", g)
	}

	for _, guildID := range guildIDs {
		cmds, err := s.ApplicationCommands(s.State.User.ID, guildID)
		if err != nil {
			return err
		}
		for _, cmd := range cmds {
			if err := s.ApplicationCommandDelete(cmd.ApplicationID, cmd.GuildID, cmd.ID); err != nil {
				return err
			}
			guild := ""
			if cmd.GuildID != "" {
				guild = " from " + cmd.GuildID
			}
			log.Printf("Deleted %s (%s)%s", cmd.Name, cmd.ID, guild)
		}
	}

	log.Println("Done cleaning up commands!")
	return nil
}
