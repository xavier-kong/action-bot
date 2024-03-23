package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

func sendCreateTodoDialog(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelData, err := s.Channel(i.ChannelID)
	if err != nil {
		fmt.Println("error with createNewTodoChannel", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I have failed you master...an error has occurred",
			},
		})
		return
	}

	if channelData.Name != "main" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("todo can only be called in main not %s", channelData.Name),
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "todo_" + i.Interaction.Member.User.ID,
			Title:    "Create new todo",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID: "Title",
							// Label:       "Title",
							Style:       discordgo.TextInputShort,
							Placeholder: "Name of your todo",
							Required:    true,
							MaxLength:   300,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:  "Deadline",
							Label:     "YYYY-MM-DD format please",
							Style:     discordgo.TextInputShort,
							Required:  true,
							MaxLength: 10,
						},
					},
				},
			},
		},
	})
}

func isValidDate(dateStr string) bool {
	layout := "2006-01-02"
	_, err := time.Parse(layout, dateStr)
	return err == nil
}

func formatDate(dateStr string) (string, error) {
	layout := "2006-01-02"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	day := t.Day()
	month := t.Month().String()
	year := t.Year()

	// Format the day suffix
	suffix := "th"
	switch day {
	case 1, 21, 31:
		suffix = "st"
	case 2, 22:
		suffix = "nd"
	case 3, 23:
		suffix = "rd"
	}

	// Format the date string
	formattedDate := t.Weekday().String() + " " + fmt.Sprintf("%d%s %s %d", day, suffix, month, year)
	return formattedDate, nil
}

func createNewTodoChannel(s *discordgo.Session, i *discordgo.InteractionCreate, data *discordgo.ModalSubmitInteractionData) {
	title := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	dateStr := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	if len(title) <= 1 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "title should be longer than 1",
			},
		})
		return
	}

	formatedDateStr, formatDateErr := formatDate(dateStr)

	if formatDateErr != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Date is invalid %s is not a proper date", dateStr),
			},
		})
		return
	}

	res, err := s.GuildChannelCreateComplex(i.GuildID, discordgo.GuildChannelCreateData{
		Name:     fmt.Sprintf("%s (%s)", title, formatedDateStr),
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: "",
	})
}

// Variables used for command line parameters
var (
	Token    string
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "todo",
			Description: "Create a new todo (creates a new channel under todo)",
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"todo": sendCreateTodoDialog,
	}
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))

	fmt.Println("Adding commands...")

	for i, v := range commands {
		cmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, "", v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}

		case discordgo.InteractionModalSubmit:
			modalSubmission := i.ModalSubmitData()
			if strings.HasPrefix(modalSubmission.CustomID, "todo") {
				createNewTodoChannel(s, i, &modalSubmission)
				// create channel here
			}
		}
	})

	defer dg.Close()
	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	fmt.Println("removing commands")
	for _, v := range registeredCommands {
		err := dg.ApplicationCommandDelete(dg.State.User.ID, "", v.ID)
		if err != nil {
			log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
		}
	}
	// Cleanly close down the Discord session.
}
