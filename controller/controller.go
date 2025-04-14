package controller

import (
	"dc-bot/controller/handler/slashcommand"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var sess *discordgo.Session
var wg sync.WaitGroup

func Start() {
	// Set log level to debug
	logrus.SetLevel(logrus.DebugLevel)

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logrus.Fatalf("Error loading .env file: %v", err)
	}

	// Create Discord session
	token := os.Getenv("TOKEN")
	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		logrus.Fatalf("Error creating Discord session: %v", err)
	}
	defer sess.Close()

	// Open Discord session
	if err := sess.Open(); err != nil {
		logrus.Fatalf("Error opening Discord session: %v", err)
	}

	// Register handlers
	slashcommand.RegisterHandlers(sess, &wg)

	// Set intents
	sess.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	logrus.Info("Bot is now running. Press CTRL-C to exit.")

	// Wait for termination signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
