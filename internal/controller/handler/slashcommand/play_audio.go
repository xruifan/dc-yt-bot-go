package slashcommand

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"io"
	"layeh.com/gopus"
	"os/exec"
	"strings"
	"sync"
)

const (
	FFmpegPath = "/usr/bin/ffmpeg"
	CHANNELS   = 2
	FRAME_RATE = 48000
	FRAME_SIZE = 960
	MAX_BYTES  = (FRAME_SIZE * 2) * 2
)

type Connection struct {
	voiceConnection *discordgo.VoiceConnection
	send            chan []int16
	playing         bool
	stopRunning     bool
	sendpcm         bool
	lock            sync.Mutex
}

type VideoInfo struct {
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
}

// GuildConnections stores the connections per guild
var GuildConnections = make(map[string]*Connection)
var gcLock sync.Mutex

func playAudio(s *discordgo.Session, i *discordgo.InteractionCreate) {
	loadingMessageEmbed := &discordgo.MessageEmbed{
		Title:       "Loading...",
		Description: "Please wait while I process your request.",
		Color:       0x00ff00, // Green color
	}
	initialResponse := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{loadingMessageEmbed},
		},
	}
	s.InteractionRespond(i.Interaction, initialResponse)

	options := i.ApplicationCommandData().Options
	if len(options) != 1 {
		message := "Please provide a valid URL."
		interactionResponse := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
			},
		}
		s.InteractionRespond(i.Interaction, interactionResponse)
		return
	}

	urlOption := options[0]

	if urlOption.Name != "url" {
		message := "Please provide a valid URL."
		interactionResponse := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
			},
		}
		s.InteractionRespond(i.Interaction, interactionResponse)
		return
	}

	guildID := i.GuildID
	gcLock.Lock()
	vs, err := joinVoiceChannel(s, i)
	if err != nil {
		logrus.Error("Error joining voice channel: ", err)
		gcLock.Unlock()
		return
	}

	connection, exists := GuildConnections[guildID]
	if !exists {
		connection = &Connection{
			voiceConnection: vs,
		}
		GuildConnections[guildID] = connection
	}
	gcLock.Unlock()

	videoInfo, err := getVideoInfo(urlOption.StringValue())
	if err != nil {
		logrus.Error("Error getting video info: ", err)
		interactionResponse := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error getting video info.",
			},
		}
		s.InteractionRespond(i.Interaction, interactionResponse)
		return
	}

	nowPlayingEmbed := createNowPlayingEmbed(videoInfo)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: nil,
		Embeds:  &[]*discordgo.MessageEmbed{nowPlayingEmbed},
	})

	streamURL, err := getStreamURL(urlOption.StringValue())
	if err != nil {
		logrus.Error("Error getting stream URL: ", err)
		interactionResponse := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error getting stream URL.",
			},
		}
		s.InteractionRespond(i.Interaction, interactionResponse)
		return
	}

	go playStream(connection, streamURL)
}

func (connection *Connection) sendPCM(voice *discordgo.VoiceConnection, pcm <-chan []int16) {
	connection.lock.Lock()
	if connection.sendpcm || pcm == nil {
		connection.lock.Unlock()
		return
	}
	connection.sendpcm = true
	connection.lock.Unlock()
	defer func() {
		connection.sendpcm = false
	}()
	encoder, err := gopus.NewEncoder(FRAME_RATE, CHANNELS, gopus.Audio)
	if err != nil {
		fmt.Println("NewEncoder error,", err)
		return
	}
	for {
		receive, ok := <-pcm
		if !ok {
			fmt.Println("PCM channel closed")
			return
		}
		opus, err := encoder.Encode(receive, FRAME_SIZE, MAX_BYTES)
		if err != nil {
			fmt.Println("Encoding error,", err)
			return
		}
		if !voice.Ready || voice.OpusSend == nil {
			fmt.Printf("Discordgo not ready for opus packets. %+v : %+v", voice.Ready, voice.OpusSend)
			return
		}
		voice.OpusSend <- opus
	}
}

func (connection *Connection) Play(ffmpeg *exec.Cmd) error {
	if connection.playing {
		return errors.New("song already playing")
	}
	connection.stopRunning = false
	out, err := ffmpeg.StdoutPipe()
	if err != nil {
		return err
	}
	buffer := bufio.NewReaderSize(out, 16384)
	err = ffmpeg.Start()
	if err != nil {
		return err
	}
	connection.playing = true
	defer func() {
		connection.playing = false
	}()
	connection.voiceConnection.Speaking(true)
	defer connection.voiceConnection.Speaking(false)
	if connection.send == nil {
		connection.send = make(chan []int16, 2)
	}
	go connection.sendPCM(connection.voiceConnection, connection.send)
	for {
		if connection.stopRunning {
			ffmpeg.Process.Kill()
			break
		}
		audioBuffer := make([]int16, FRAME_SIZE*CHANNELS)
		err = binary.Read(buffer, binary.LittleEndian, &audioBuffer)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
		connection.send <- audioBuffer
	}
	return nil
}

func (connection *Connection) Stop() {
	connection.stopRunning = true
	connection.playing = false
}

func playStream(connection *Connection, streamURL string) {
	ffmpeg := exec.Command("ffmpeg", "-i", streamURL, "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1")
	err := connection.Play(ffmpeg)
	if err != nil {
		logrus.Error("Error playing stream: ", err)
	}
}

func getStreamURL(url string) (string, error) {
	cmd := exec.Command("yt-dlp", "-x", "-f", "worstaudio", "-g", url)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func createNowPlayingEmbed(info *VideoInfo) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Now Playing",
		Description: fmt.Sprintf("[%s](%s)", info.Title, info.URL),
		Color:       0xff0000, // Red color
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: info.Thumbnail,
		},
	}
}

func getVideoInfo(url string) (*VideoInfo, error) {
	cmd := exec.Command("yt-dlp", "--no-playlist", "-j", url)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var info VideoInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, err
	}

	return &info, nil
}
