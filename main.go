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
	token     = os.Getenv("BOT_TOKEN")
	pubKeyHex = os.Getenv("PUBKEY")

	wsMode   = flag.Bool("ws", false, "run in websocket mode instead of listening for incoming webhooks")
	register = flag.Bool("register", false, "register bot commands with discord; add the -cleanup flag to first remove any old commands")
	cleanup  = flag.Bool("cleanup", false, "when running with -register, also first remove any previously registered commands")
)

func init() {
	flag.Parse()
}

func main() {
	var err error
	pubKey, err = parsePubKey(pubKeyHex)
	if err != nil {
		log.Println(err)
	}

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

	if *register {
		app, err := s.Application("@me")
		if err != nil {
			return err
		}

		if *cleanup {
			if err := cleanupCommands(s, app.ID); err != nil {
				return err
			}
		}
		return registerCommands(s, app.ID)
	}

	if *wsMode {
		s.AddHandler(interactionHandle)
		err = s.Open()
		if err != nil {
			return err
		}
		defer s.Close()

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		log.Println("Bot now connected and ready. Press Ctrl+C to exit...")
		<-stop
	} else {
		listenAndServe()
	}

	return nil
}
