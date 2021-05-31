package chat

import (
	"fmt"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var Users map[string]*websocket.Conn

func init() {
	Users = map[string]*websocket.Conn{}
}

type User struct {
	nick string
	peer *websocket.Conn
}

func NewUser(n string, p *websocket.Conn) *User {
	return &User{nick: n, peer: p}
}

const usernameHasBeenTaken = "username %s is already taken. please retry with a different name"
const welcome = "Welcome %s!"
const chat = "%s: %s"
const left = "%s: has left the chat."

func (u *User) Start() {
	if _, ok := Users[u.nick]; ok {
		err := u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(usernameHasBeenTaken, u.nick)))
		if err != nil {
			log.Error("failed to write message", err)
		}

		u.peer.Close()

		return
	}

	Users[u.nick] = u.peer

	err := u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(welcome, u.nick)))
	if err != nil {
		log.Error("failed to write message", err)
	}

	go func() {
		log.Println("user joined", u.nick)

		for {
			_, msg, err := u.peer.ReadMessage()
			if err != nil {
				_, ok := err.(*websocket.CloseError)
				if ok {
					log.Println("connection closed by user")
					u.disconnect()
				}

				return
			}

			u.SendToChat(fmt.Sprintf(chat, u.nick, string(msg)))
		}
	}()
}

func (u *User) SendToChat(msg string) {
	for nick, conn := range Users {
		if nick == u.nick {
			continue
		}

		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Error("failed to write message", err)
		}
	}
}

func (u *User) disconnect() {
	u.SendToChat(fmt.Sprintf(left, u.nick))
	u.peer.Close()
	delete(Users, u.nick)
}
