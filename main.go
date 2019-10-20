package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cirias/tgbot"
	"github.com/pkg/errors"
	"github.com/tarm/serial"
)

var keyboard = [][]*tgbot.KeyboardButton{
	[]*tgbot.KeyboardButton{
		&tgbot.KeyboardButton{
			Text: "Big Off",
		},
		&tgbot.KeyboardButton{
			Text: "Small Off",
		},
	},
	[]*tgbot.KeyboardButton{
		&tgbot.KeyboardButton{
			Text: "Big On",
		},
		&tgbot.KeyboardButton{
			Text: "Small On",
		},
	},
}

var messageCommandMap = map[string][]byte{}

func init() {
	messageCommandMap[keyboard[0][0].Text] = []byte{'a'}
	messageCommandMap[keyboard[0][1].Text] = []byte{'A'}
	messageCommandMap[keyboard[1][0].Text] = []byte{'b'}
	messageCommandMap[keyboard[1][1].Text] = []byte{'B'}
}

func main() {
	port := flag.String("port", "/dev/ttyAMA0", "path of serial port device")
	token := flag.String("token", "", "telegram bot token")
	userIds := flag.String("users", "", "id0[,id1[...]] allowed users' id")

	flag.Parse()

	app, err := newApp(*port, *token, *userIds)
	if err != nil {
		log.Fatalln(err)
	}

	log.Fatalln(app.run())
}

type App struct {
	port  *serial.Port
	bot   *tgbot.Bot
	users []int64
}

func newApp(portName, token, userIds string) (*App, error) {
	users, err := parseUserIds(userIds)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse users")
	}

	port, err := openPort(portName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open port")
	}

	bot := tgbot.NewBot(token)

	return &App{port: port, bot: bot, users: users}, nil
}

func openPort(portName string) (*serial.Port, error) {
	if portName == "" {
		log.Println("WARNING: port name is not set, running in debug mode")
		return nil, nil
	}

	return serial.OpenPort(&serial.Config{Name: portName, Baud: 115200})
}

func parseUserIds(userIds string) ([]int64, error) {
	idstrs := strings.Split(userIds, ",")
	ids := make([]int64, 0, len(idstrs))
	for _, str := range idstrs {
		if str == "" {
			continue
		}

		id, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse string to int64: %s", str)
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (a *App) run() error {
	params := &tgbot.GetUpdatesParams{
		Offset:  0,
		Limit:   10,
		Timeout: 10,
	}

	for {
		updates, err := a.getUpdates(params)
		if err != nil {
			return errors.Wrapf(err, "could not get updates")
		}

		for _, u := range updates {
			if err := a.handleUpdate(u); err != nil {
				log.Println("could not handle message", err)
			}
		}

		if len(updates) > 0 {
			params.Offset = updates[len(updates)-1].Id + 1
		}
	}
}

func contains(arr []int64, n int64) bool {
	for _, a := range arr {
		if a == n {
			return true
		}
	}

	return false
}

func (a *App) handleUpdate(update *tgbot.Update) error {
	if !contains(a.users, update.Message.From.Id) {
		return errors.Errorf("invalid user: %d", update.Message.From.Id)
	}

	if update.Message.Text == "/ping" {
		_, err := a.sendMessage(&tgbot.SendMessageParams{
			ChatId: update.Message.Chat.Id,
			Text:   "pong",
		})
		return errors.Wrapf(err, "could not send message")
	}

	if update.Message.Text == "/start" {
		_, err := a.sendMessage(&tgbot.SendMessageParams{
			ChatId:      update.Message.Chat.Id,
			Text:        "What do you want?",
			ReplyMarkup: &tgbot.ReplyKeyboardMarkup{Keyboard: keyboard},
		})
		return errors.Wrapf(err, "could not send message")
	}

	cmd, ok := messageCommandMap[update.Message.Text]
	if ok {
		messageText := "Sounds good!"

		if err := a.sendCommand(cmd); err != nil {
			messageText = fmt.Sprintf("Oops! %s", err)
		}

		_, err := a.sendMessage(&tgbot.SendMessageParams{
			ChatId: update.Message.Chat.Id,
			Text:   messageText,
		})

		return errors.Wrapf(err, "could not send message")
	}

	return errors.Errorf("unknown message: %s", update.Message.Text)
}

func (a *App) sendCommand(cmd []byte) error {
	log.Printf("sending command: %s", cmd)

	if a.port == nil {
		return errors.Errorf("debug: command not sent")
	}

	if _, err := a.port.Write(cmd); err != nil {
		return errors.Wrapf(err, "could not write to port")
	}

	// TODO do we need this?
	if err := a.port.Flush(); err != nil {
		return errors.Wrapf(err, "could not flush port")
	}

	return nil
}

func (a *App) getUpdates(params *tgbot.GetUpdatesParams) (updates []*tgbot.Update, err error) {
	err = willRetry(func() error {
		var err error
		updates, err = a.bot.GetUpdates(params)
		return err
	}, 4)
	return
}

func (a *App) sendMessage(params *tgbot.SendMessageParams) (message *tgbot.Message, err error) {
	err = willRetry(func() error {
		var err error
		message, err = a.bot.SendMessage(params)
		return err
	}, 4)
	return
}

func willRetry(fn func() error, n int) error {
	err := fn()
	for i := 0; i < n; i++ {
		if err == nil {
			break
		}
		time.Sleep(time.Duration(math.Pow10(i)) * time.Millisecond * 100) // 0.1, 1, 10, 100

		log.Printf("retry %d: %v\n", i, err)
		err = fn()
	}

	return err
}
