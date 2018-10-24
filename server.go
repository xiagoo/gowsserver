package gowsserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)

type Message struct {
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
}

type SocketServer struct {
	Clients    map[*Client]bool //存储client
	Register   chan *Client     //注册
	UnRegister chan *Client     //卸载
	Broadcast  chan []byte      //广播消息
}

var socketServer *SocketServer
var instanceOnce sync.Once

// GetSocketServerInstance 获取SocketServer全局实例
func GetSocketServerInstance() *SocketServer {
	if socketServer != nil {
		return socketServer
	}
	instanceOnce.Do(func() {
		socketServer = NewSocketServer()
	})

	return socketServer
}

type Client struct {
	ID      string          //ClientID
	UserID  int             //用户ID
	Socket  *websocket.Conn //Socket连接
	Message chan []byte     //发送给Client的消息
}

func NewSocketServer() *SocketServer {
	return &SocketServer{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		UnRegister: make(chan *Client),
		Broadcast:  make(chan []byte),
	}
}

func (s *SocketServer) Start() {
	for {
		select {
		case c := <-s.Register:
			s.Clients[c] = true
			message, _ := json.Marshal(&Message{Content: "A new socket has connected."})
			s.PushAllWithout(message, c)
			go s.Read(c)
			go s.Push(c)
		case c := <-s.UnRegister:
			if _, ok := s.Clients[c]; ok {
				c.Socket.Close()
				delete(s.Clients, c)
				message, _ := json.Marshal(&Message{Content: "A new socket has disconnected."})
				s.PushAll(message)
			}
		case message := <-s.Broadcast:
			for c := range s.Clients {
				select {
				case c.Message <- message:
					fmt.Printf("Broadcast\n")
				default:
					c.Socket.Close()
					delete(s.Clients, c)
					fmt.Printf("stop\n")
				}
			}
		}
		fmt.Printf("cnt=%d\n", len(s.Clients))
	}
}

func (s *SocketServer) Close(c *Client) {
	c.Socket.Close()
	close(c.Message)
	delete(s.Clients, c)
}

func (s *SocketServer) Read(c *Client) {
	defer func() {
		s.UnRegister <- c
	}()

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			s.UnRegister <- c
			c.Socket.Close()
			break
		}
		jsonMsg, _ := json.Marshal(&Message{Sender: c.ID, Content: string(message)})
		s.Broadcast <- jsonMsg
	}
}

func (s *SocketServer) Push(c *Client) {
	defer func() {
		s.UnRegister <- c
	}()
	for {
		select {
		case message := <-c.Message:
			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (s *SocketServer) PushAll(message []byte) {
	for c := range s.Clients {
		c.Message <- message
		s.Push(c)
	}
}

func (s *SocketServer) Heartbeat() {

}

func (s *SocketServer) GetConn(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}).Upgrade(w, r, nil)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	return conn
}

func (s *SocketServer) PushAllWithout(message []byte, ignore *Client) {
	for c := range s.Clients {
		if c != ignore {
			c.Message <- message
		}
		fmt.Printf("ID=%s, Message=%v\n", c.ID, c.Message)
	}
}

func WSHandler(w http.ResponseWriter, r *http.Request) {
	s := GetSocketServerInstance()
	conn := s.GetConn(w, r)
	c := &Client{ID: uuid.NewV4().String(), UserID: GetUserID(r), Socket: conn, Message: make(chan []byte)}
	s.Register <- c
}

func GetUserID(r *http.Request) int {
	return 0
}

func GetAuth(r *http.Request) bool {
	return true
}

func PushHandler(w http.ResponseWriter, r *http.Request) {
	s := GetSocketServerInstance()
	s.Broadcast <- []byte(r.FormValue("user_id"))
}

func Run(addr, wspath, pushPath string) {
	fmt.Println("Starting application...")
	s := GetSocketServerInstance()
	go s.Start()
	http.HandleFunc(fmt.Sprintf("/%s", wspath), WSHandler)
	http.HandleFunc(fmt.Sprintf("/%s", pushPath), PushHandler)
	http.ListenAndServe(addr, nil)
}
