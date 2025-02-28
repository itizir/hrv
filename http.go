package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"
)

var pubKey ed25519.PublicKey

func parsePubKey(data string) (ed25519.PublicKey, error) {
	pk, err := hex.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	} else if len(pk) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key: invalid length")
	}
	return pk, nil
}

func listenAndServe() {
	http.HandleFunc("/", httpHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	if len(pubKey) != ed25519.PublicKeySize || !discordgo.VerifyInteraction(r, pubKey) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	i := &discordgo.InteractionCreate{}
	if err := json.NewDecoder(r.Body).Decode(i); err != nil {
		log.Println("failed to decode", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if i.Type == discordgo.InteractionPing {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(discordgo.InteractionResponse{
			Type: discordgo.InteractionResponsePong,
		})
		if err != nil {
			log.Println("error sending response:", err)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
	go func() {
		s, err := discordgo.New("Bot " + token)
		if err != nil {
			log.Println("failed to create session", err)
			return
		}
		interactionHandle(s, i)
	}()
}
