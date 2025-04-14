package slashcommand

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"sync"
)

func RegisterHandlers(s *discordgo.Session, wg *sync.WaitGroup) {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "play",
			Description: "Play audio from a given URL",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "url",
					Description: "The URL of the audio to play",
					Required:    true,
				},
			},
		},
		{
			Name:        "join",
			Description: "Join a voice channel",
		},
		{
			Name:        "leave",
			Description: "Leave the voice channel",
		},
	}

	commandIDs := make(map[string]string)

	// Get guilds the bot is in to register the slash command
	guilds, err := s.UserGuilds(0, "", "", true)
	if err != nil {
		// Handle error
		return
	}

	// Register the slash command for each guild
	logrus.Debug("Registering slash command...")
	for _, guild := range guilds {
		guildID := guild.ID
		createdCommands, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, commands)
		if err != nil {
			logrus.Fatal(err)
		}

		// Store the command IDs
		for _, cmd := range createdCommands {
			commandIDs[cmd.Name] = cmd.ID
		}
	}

	logrus.Debug("Slash command registered successfully.")

	// Use anonymous function to pass wg
	s.AddHandler(func(s *discordgo.Session, i interface{}) {
		// Type assertion to handle specific event type
		if interaction, ok := i.(*discordgo.InteractionCreate); ok {
			playAudioHandler(s, interaction, wg)
			joinVoiceChannelHandler(s, interaction, wg)
			leaveVoiceChannelHandler(s, interaction, wg)
		}
	})
}

func playAudioHandler(s *discordgo.Session, i *discordgo.InteractionCreate, wg *sync.WaitGroup) {
	if i.ApplicationCommandData().Name == "play" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			playAudio(s, i)
		}()
	}
}

func joinVoiceChannelHandler(s *discordgo.Session, i *discordgo.InteractionCreate, wg *sync.WaitGroup) {
	if i.ApplicationCommandData().Name == "join" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			joinVoiceChannel(s, i)
		}()
	}
}

func leaveVoiceChannelHandler(s *discordgo.Session, i *discordgo.InteractionCreate, wg *sync.WaitGroup) {
	if i.ApplicationCommandData().Name == "leave" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaveVoiceChannel(s, i)
		}()
	}
}
