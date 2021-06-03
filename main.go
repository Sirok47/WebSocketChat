package main

import (
	"WebSocketChat/chat"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strings"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func main() {
	TokenValidation := middleware.JWTWithConfig(middleware.JWTConfig{
		SigningKey: []byte(chat.Key),
	})

	e := echo.New()

	e.GET("/", home, TokenValidation)

	e.GET("/chat", websocketHandler)

	e.POST("/signup", chat.SignUp)

	e.GET("/login", chat.LogIn)

	e.Logger.Fatal(e.Start(":8080"))
}

func getToken(c echo.Context) *chat.Session {
	header := c.Request().Header["Authorization"]
	header = strings.Split(header[0], " ")
	token, err := jwt.ParseWithClaims(header[1], &chat.Session{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(chat.Key), nil
	})
	if err != nil {
		return nil
	}
	if claims, ok := token.Claims.(*chat.Session); ok && token.Valid {
		return claims
	}
	return nil
}

func websocketHandler(ctx echo.Context) error {
	token := getToken(ctx)

	peer, err := upgrader.Upgrade(ctx.Response().Writer, ctx.Request(), nil)
	if err != nil {
		log.Error("websocket conn failed ", err)
		return err
	}

	chatSession := chat.NewSession(token.Nick, peer)
	chatSession.Start()
	return nil
}

func home(ctx echo.Context) error {
	homeTemplate, err := template.ParseFiles("page.html")
	if homeTemplate == nil || err != nil {
		log.Fatal("Unable to parse html, ", err)
	}
	err = homeTemplate.Execute(ctx.Response().Writer, "ws://localhost:8080/chat")
	if err != nil {
		log.Fatal("Unable to create page, ", err)
	}
	return nil
}
