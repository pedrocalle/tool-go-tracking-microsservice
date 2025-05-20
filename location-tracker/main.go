// main.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
)

type Location struct {
	PhoneID   string  `json:"phone_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

var (
	conn      *pgx.Conn
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Location)
	upgrader  = websocket.Upgrader{}
)

func main() {
	var err error
	conn, err = pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Erro ao conectar ao banco:", err)
	}
	defer conn.Close(context.Background())

	err = createTableIfNotExists()
	if err != nil {
		log.Fatal("Erro ao criar tabela:", err)
	}

	http.HandleFunc("/ws", handleWS)
	http.HandleFunc("/update-location", handleUpdate)

	go broadcastLocations()

	log.Println("Servidor rodando em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createTableIfNotExists() error {
	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS locations (
		id SERIAL PRIMARY KEY,
		phone_id TEXT NOT NULL,
		latitude DOUBLE PRECISION NOT NULL,
		longitude DOUBLE PRECISION NOT NULL,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`)
	return err
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro no upgrade do WS:", err)
		return
	}
	defer ws.Close()

	clients[ws] = true

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			delete(clients, ws)
			break
		}
	}
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var loc Location
	err := json.NewDecoder(r.Body).Decode(&loc)
	if err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	loc.Timestamp = time.Now().Unix()

	_, err = conn.Exec(context.Background(),
		`INSERT INTO locations (phone_id, latitude, longitude, updated_at) VALUES ($1, $2, $3, to_timestamp($4))`,
		loc.PhoneID, loc.Latitude, loc.Longitude, loc.Timestamp)
	if err != nil {
		http.Error(w, "Erro ao salvar localização", http.StatusInternalServerError)
		return
	}

	broadcast <- loc

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Localização atualizada"))
}

func broadcastLocations() {
	for {
		loc := <-broadcast
		msg, _ := json.Marshal(loc)

		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}
