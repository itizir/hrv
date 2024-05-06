package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

var (
	token    = os.Getenv("BOT_TOKEN")
	register = flag.Bool("register", false, "register bot commands with discord; add the -cleanup flag to first remove any old commands")
	cleanup  = flag.Bool("cleanup", false, "when running with -register, also first remove any previously registered commands")
)

func init() {
	flag.Parse()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if token == "" {
		return errors.New("BOT_TOKEN not set")
	}
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return err
	}

	s.AddHandler(interactionHandler)

	err = s.Open()
	if err != nil {
		return err
	}
	defer s.Close()

	if *register {
		if *cleanup {
			if err := cleanupCommands(s); err != nil {
				return err
			}
		}
		return registerCommands(s)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Bot now connected and ready. Press Ctrl+C to exit...")
	<-stop

	return nil
}
