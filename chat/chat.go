package chat

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var (
	db    *redis.Client
	Users map[string]*websocket.Conn
)

func init() {
	Users = make(map[string]*websocket.Conn)
	db = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	db.FlushDB(context.Background())
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
	_, err := db.HGet(context.Background(), "Users", u.nick).Result()
	if err != redis.Nil {
		err := u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(usernameHasBeenTaken, u.nick)))
		if err != nil {
			log.Error("failed to write message", err)
		}

		u.peer.Close()

		return
	}

	Users[u.nick] = u.peer

	err = db.HSet(context.Background(), "Users", u.nick, 0).Err()
	if err != nil {
		log.Error("failed to add new user ", err)
	}

	err = u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(welcome, u.nick)))
	if err != nil {
		log.Error("failed to write message ", err)
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
	users := db.HGetAll(context.Background(), "Users")
	for nick, _ := range users.Val() {
		if nick == u.nick {
			continue
		}
		conn := Users[nick]
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Error("failed to write message", err)
		}
	}
}

func (u *User) disconnect() {
	u.SendToChat(fmt.Sprintf(left, u.nick))
	u.peer.Close()
	db.Del(context.Background(), u.nick)
}
