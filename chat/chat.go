package chat

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const Key = "ultrasecretsignigkey"

var (
	rdb   *redis.Client
	Users map[string]*websocket.Conn
	pdb   *pgx.Conn
)

func init() {
	Users = make(map[string]*websocket.Conn)
	var err error
	pdb, err = pgx.Connect(context.Background(), "postgres://maks:glazirovanniisirok@127.0.0.1:5432/accounts")
	if err != nil {
		log.Fatal("Unable to connect to database: ", err)
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	rdb.FlushDB(context.Background())
}

type Session struct {
	Nick string
	peer *websocket.Conn
	jwt.StandardClaims
}

type Account struct {
	nick     string `json:"Nick"`
	password string `json:"Password"`
}

func NewSession(n string, p *websocket.Conn) *Session {
	return &Session{Nick: n, peer: p}
}

const usernameHasBeenTaken = "username %s is already taken. please retry with a different name"
const welcome = "Welcome %s!"
const chat = "%s: %s"
const left = "%s: has left the chat."

func (u *Session) Start() {
	_, err := rdb.HGet(context.Background(), "Users", u.Nick).Result()
	if err != redis.Nil {
		err := u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(usernameHasBeenTaken, u.Nick)))
		if err != nil {
			log.Error("failed to write message", err)
		}

		u.peer.Close()

		return
	}

	Users[u.Nick] = u.peer

	err = rdb.HSet(context.Background(), "Sessions", u.Nick, 0).Err()
	if err != nil {
		log.Error("failed to add new user ", err)
	}

	err = u.peer.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(welcome, u.Nick)))
	if err != nil {
		log.Error("failed to write message ", err)
	}

	go func() {
		log.Println("user joined", u.Nick)

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

			u.SendToChat(fmt.Sprintf(chat, u.Nick, string(msg)))
		}
	}()
}

func (u *Session) SendToChat(msg string) {
	users := rdb.HGetAll(context.Background(), "Users")
	for nick, _ := range users.Val() {
		if nick == u.Nick {
			continue
		}
		conn := Users[nick]
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Error("failed to write message", err)
		}
	}
}

func (u *Session) disconnect() {
	u.SendToChat(fmt.Sprintf(left, u.Nick))
	u.peer.Close()
	rdb.Del(context.Background(), u.Nick)
}

func SignUp(ctx echo.Context) error {
	user := &Account{}
	err := ctx.Bind(user)
	if err != nil {
		log.Error("Error while parsing json: ", err)
		return err
	}
	res, err := pdb.Exec(context.Background(), "insert into accounts (nick,password) values ($1,$2)", user.nick, user.password)
	if err != nil {
		log.Error("Failed to create new user: ", err)
		return err
	}
	if res.RowsAffected() == 0 {
		return ctx.String(http.StatusForbidden, "User with such nick is already existing.")
	}
	return ctx.String(http.StatusCreated, "New user created.")
}

func LogIn(ctx echo.Context) error {
	user := &Account{}
	trueUser := &Account{}
	err := ctx.Bind(user)
	if err != nil {
		log.Error("Error while parsing json: ", err)
		return err
	}
	res, err := pdb.Query(context.Background(), "select * from accounts where nick = $1", user.nick)
	if res != nil {
		defer res.Close()
	} else {
		log.Error("Error while query: ", err)
	}
	if err != nil {
		log.Error("Error while query: ", err)
	}
	for res.Next() {
		err = res.Scan(&trueUser.nick, &trueUser.password)
		if err != nil {
			log.Error("Error while query: ", err)
			return err
		}
	}
	if trueUser.password == user.password {
		token := jwt.New(jwt.SigningMethodHS256)
		claims := token.Claims.(jwt.MapClaims)
		claims["Nick"] = trueUser.nick
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		t, err := token.SignedString([]byte(Key))
		if err != nil {
			log.Error("Failed to sign token: ", err)
			return err
		}
		return ctx.String(http.StatusOK, t)
	}
	return echo.ErrUnauthorized
}
