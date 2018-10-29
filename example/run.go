package main

import (
	"fmt"
	"net/http"

	"strconv"

	uuid "github.com/satori/go.uuid"
	"github.com/xiagoo/gowsserver"
)

func main() {
	fmt.Println("Starting application...")
	s := gowsserver.GetSocketServerInstance()
	go s.Start()
	http.HandleFunc("/ws", WSHandler)
	http.HandleFunc("/push", PushHandler)
	http.HandleFunc("/push_all", PushAllHandler)
	http.ListenAndServe(":7777", nil)
}

func WSHandler(w http.ResponseWriter, r *http.Request) {
	s := gowsserver.GetSocketServerInstance()
	conn := s.GetConn(w, r)
	c := &gowsserver.Client{ID: uuid.NewV4().String(), UserID: GetUserID(r), Socket: conn, Message: make(chan []byte)}
	s.Register <- c
}

func PushHandler(w http.ResponseWriter, r *http.Request) {
	s := gowsserver.GetSocketServerInstance()
	for c := range s.Clients {
		if c.UserID == 135246 {
			c.Message <- []byte(fmt.Sprintf("test->user_id:%s", r.Header.Get("user_id")))
		}
	}
}

func PushAllHandler(w http.ResponseWriter, r *http.Request) {
	s := gowsserver.GetSocketServerInstance()
	s.Broadcast <- []byte(r.FormValue("user_id"))
}

func GetUserID(r *http.Request) int {
	userID, err := strconv.Atoi(r.Header.Get("user_id"))
	if err != nil {
		return 0
	}
	return userID
}

func GetAuth(r *http.Request) bool {
	return true
}
