package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Location struct {
	DeliveryManID   string  `json:"delivery_man_id"`
	OrderId string `json:"order_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

var (
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func broadcastLocation(loc Location) {
	message, _ := json.Marshal(loc)

	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Println("erro ao enviar:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("erro ao fazer upgrade:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	log.Println("cliente conectado")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	clientsMu.Lock()
	delete(clients, conn)
	clientsMu.Unlock()
	conn.Close()
	log.Println("cliente desconectado")
}

func handleIncomingLocation(w http.ResponseWriter, r *http.Request) {
	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	go broadcastLocation(loc)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/location", handleIncomingLocation)

	log.Println("servidor escutando em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
