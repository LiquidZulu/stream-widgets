package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/fatih/color"
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"stream-widgets/util/badwords"
)

type Message struct {
	Platform      string `json:"platform"`
	Username      string `json:"username"`
	Content       string `json:"content"`
	Timestamp     string `json:"timestamp"`
	Color         string `json:"color"`
	PlatformColor string `json:"platformColor"`
}

type ChatService struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan Message
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func NewChatService() *ChatService {
	return &ChatService{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan Message),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run starts the chat service
func (cs *ChatService) Run(key string) {
	for {
		select {
		case client := <-cs.register:
			cs.mu.Lock()
			cs.clients[client] = true
			cs.mu.Unlock()
			log.Printf("[%s] New client connected. Total clients: %d", color.GreenString(key), len(cs.clients))
		case client := <-cs.unregister:
			cs.mu.Lock()
			delete(cs.clients, client)
			cs.mu.Unlock()
			log.Printf("[%s] Client disconnected. Total clients: %d", color.GreenString(key), len(cs.clients))
		case message := <-cs.broadcast:

			if badwords.IsBad(message.Content) {
				log.Printf("[%s] %s from %s: %s", color.GreenString(key), color.RedString("BANNED MESSAGE"), color.GreenString(message.Username), message.Content)
				continue
			}

			cs.mu.Lock()
			for client := range cs.clients {
				err := client.WriteJSON(message)
				if err != nil {
					log.Printf("[%s] Error sending message to client: %v", color.GreenString(key), err)
					client.Close()
					delete(cs.clients, client)
				}
			}
			cs.mu.Unlock()
		}
	}
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from localhost for testing
	},
}

func GetKey(query url.Values) string {
	platforms := []string{"twitch", "youtube"}
	var keys []string

	for _, platform := range platforms {
		if value := query.Get(platform); value != "" {
			keys = append(keys, value)
		}
	}

	if len(keys) == 0 {
		return ""
	}

	return strings.Join(keys, ",")
}

func twitchListener(channel string, service *ChatService) {
	twitchClient := twitch.NewAnonymousClient()
	twitchClient.Join(channel)
	twitchClient.OnPrivateMessage(func(message twitch.PrivateMessage) {
		chatMessage := Message{
			Platform:      "\uf1e8",
			Username:      message.User.DisplayName,
			Content:       message.Message,
			Timestamp:     message.Time.Format(time.RFC3339),
			Color:         message.User.Color,
			PlatformColor: "#9045FF",
		}
		service.broadcast <- chatMessage
	})

	go func() {
		if err := twitchClient.Connect(); err != nil {
			log.Fatalf("Twitch IRC connection error: %v", err)
		}
	}()
}

type YouTubeMessage struct {
	Author struct {
		Name string `json:"name"`
	} `json:author`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func youtubeListener(channelID string, workerURL string, service *ChatService) {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0 // retry indefinitely

	for {
		err := connectYouTubeWebSocket(channelID, workerURL, service)
		if err != nil {
			log.Printf("YouTube WebSocket error, retrying: %v", err)
			time.Sleep(b.NextBackOff())
			continue
		}
		b.Reset()
	}
}

func connectYouTubeWebSocket(channelID string, workerURL string, service *ChatService) error {
	wsURL := fmt.Sprintf("%s/c/%s", workerURL, channelID)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var ytMessage YouTubeMessage
		if err := json.Unmarshal(message, &ytMessage); err != nil {
			log.Printf("Failed to parse YouTube message: %v", err)
			continue
		}
		chatMessage := Message{
			Platform:      "\uf16a",
			Username:      ytMessage.Author.Name,
			Content:       ytMessage.Message,
			Timestamp:     ytMessage.Timestamp,
			Color:         "#ffffff",
			PlatformColor: "#ff0000",
		}
		service.broadcast <- chatMessage
	}
}

type Route struct {
	Path        string
	HTTPHandler http.HandlerFunc
	WSHandler   func(http.ResponseWriter, *http.Request, map[string]*ChatService)
}

func GetChatRoutes() []Route {
	services := make(map[string]*ChatService)
	return []Route{
		{
			Path: "/chat",
			HTTPHandler: func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				twitchChannel := q.Get("twitch")
				youtubeVideoID := q.Get("youtube")
				key := GetKey(q)
				_, ok := services[key]
				if !ok {
					service := NewChatService()
					services[key] = service
					go service.Run(key)
					if twitchChannel != "" {
						twitchListener(twitchChannel, service)
					}
					if youtubeVideoID != "" {
						go youtubeListener(youtubeVideoID, "ws://localhost:8787", service)
					}
				}
				http.ServeFile(w, r, "./widgets/chat/chat.html")
			},
		},
		{
			Path: "/ws/chat",
			WSHandler: func(w http.ResponseWriter, r *http.Request, services map[string]*ChatService) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					log.Printf("WebSocket upgrade error: %v", err)
					return
				}
				q := r.URL.Query()
				key := GetKey(q)
				service, ok := services[key]
				if !ok {
					service = NewChatService()
					services[key] = service
					go service.Run(key)
					if q.Get("twitch") != "" {
						twitchListener(q.Get("twitch"), service)
					}
					if q.Get("youtube") != "" {
						go youtubeListener(q.Get("youtube"), "ws://localhost:8787", service)
					}
				}
				service.register <- conn
				defer func() {
					service.unregister <- conn
					conn.Close()
				}()
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						log.Printf("WebSocket read error: %v", err)
						return
					}
				}
			},
		},
	}
}

func RegisterChatRoutes(router *mux.Router) {
	services := make(map[string]*ChatService)
	for _, route := range GetChatRoutes() {
		if route.HTTPHandler != nil {
			router.HandleFunc(route.Path, route.HTTPHandler)
		}
		if route.WSHandler != nil {
			router.HandleFunc(route.Path, func(w http.ResponseWriter, r *http.Request) {
				route.WSHandler(w, r, services)
			})
		}
	}
}
