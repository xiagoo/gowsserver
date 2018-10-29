package gowsserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"time"

	"github.com/gorilla/websocket"
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
	mutex      sync.Mutex       //锁
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
	Closed  bool            //是否关闭
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
			go s.Read(c)
			go s.Push(c)
			//go s.Heartbeat(c)
			fmt.Printf("Socket Client Register   ID:%s Total:%d\n", c.ID, len(s.Clients))

		case c := <-s.UnRegister:
			if _, ok := s.Clients[c]; ok {
				s.Close(c)
				fmt.Printf("Socket Client UnRegister ID:%s Total:%d\n", c.ID, len(s.Clients))
			}
		case message := <-s.Broadcast:
			s.PushAll(message)
		}
	}
}

func (s *SocketServer) Close(c *Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !c.Closed {
		close(c.Message)
		delete(s.Clients, c)
		c.Closed = true
	}
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
		c.Message <- jsonMsg
	}
}

func (s *SocketServer) Push(c *Client) {
	defer func() {
		s.UnRegister <- c
	}()
	for {
		select {
		case message, ok := <-c.Message:
			if !ok {
				c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (s *SocketServer) PushAll(message []byte) {
	for c := range s.Clients {
		select {
		case c.Message <- message:
		default:
			s.Close(c)
			fmt.Printf("Socket Client Close ID:%s Total:%d\n", c.ID, len(s.Clients))
		}
	}
}

func (s *SocketServer) Heartbeat(c *Client) {
	for {
		time.Sleep(2 * time.Second)
		if err := c.Socket.WriteMessage(websocket.TextMessage, []byte("heartbeat from server")); err != nil {
			fmt.Println("heartbeat fail")
			s.Close(c)
			break
		}
	}
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
