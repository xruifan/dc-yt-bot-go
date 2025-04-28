package slashcommand

import (
	"errors"
	"github.com/bwmarrin/discordgo"
)

func getUserVoiceState(s *discordgo.Session, guildID, userID string) (*discordgo.VoiceState, error) {
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return nil, err
	}

	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs, nil
		}
	}
	return nil, nil
}

func joinVoiceChannel(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.VoiceConnection, error) {
	// Send a thinking response
	initialResponse := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
	s.InteractionRespond(i.Interaction, initialResponse)

	userID := i.Member.User.ID
	guildID := i.GuildID

	voiceState, err := getUserVoiceState(s, guildID, userID)
	if err != nil {
		return nil, err
	}

	if voiceState == nil {
		content := "You must be in a voice channel to use this command."
		interactionResponse := &discordgo.WebhookEdit{
			Content: &content,
		}
		s.InteractionResponseEdit(i.Interaction, interactionResponse)
		return nil, errors.New("user not in a voice channel")
	}

	voiceChannelID := voiceState.ChannelID
	vs, err := s.ChannelVoiceJoin(guildID, voiceChannelID, false, true)

	if err != nil {
		return nil, err
	}

	content := "Hello, I'm ready to play some music!"
	interactionResponse := &discordgo.WebhookEdit{
		Content: &content,
	}
	s.InteractionResponseEdit(i.Interaction, interactionResponse)

	return vs, nil
}

func leaveVoiceChannel(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	// Send a thinking response
	initialResponse := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
	s.InteractionRespond(i.Interaction, initialResponse)

	userID := i.Member.User.ID
	guildID := i.GuildID

	voiceState, err := getUserVoiceState(s, guildID, userID)
	if err != nil || voiceState == nil {
		content := "You must be in a voice channel to use this command."
		interactionResponse := &discordgo.WebhookEdit{
			Content: &content,
		}
		s.InteractionResponseEdit(i.Interaction, interactionResponse)
		return errors.New("user not in a voice channel")
	}

	voiceChannelID := voiceState.ChannelID
	vs, err := s.ChannelVoiceJoin(guildID, voiceChannelID, false, true)
	if err != nil {
		return err
	}
	vs.Disconnect()

	content := "Bye bye!"
	interactionResponse := &discordgo.WebhookEdit{
		Content: &content,
	}
	s.InteractionResponseEdit(i.Interaction, interactionResponse)

	return nil
}
