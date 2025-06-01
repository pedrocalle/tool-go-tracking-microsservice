package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

type Location struct {
	DeliveryManID string  `json:"delivery_man_id"`
	OrderId       string  `json:"order_id"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     int64   `json:"timestamp"`
}

var (
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	db        *sql.DB
)

func initDB() {
	var err error

	connStr := "host=localhost port=5432 user=postgres password=1234 dbname=toolgo_dev sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Erro ao conectar ao banco de dados:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Erro ao pingar o banco de dados:", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS locations (
		id SERIAL PRIMARY KEY,
		delivery_man_id TEXT NOT NULL,
		order_id TEXT NOT NULL,
		latitude DOUBLE PRECISION,
		longitude DOUBLE PRECISION,
		timestamp BIGINT
	);`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatal("Erro ao criar tabela:", err)
	}
}

func saveLocationToDB(loc Location) {
	stmt := `INSERT INTO locations (delivery_man_id, order_id, latitude, longitude, timestamp) VALUES ($1, $2, $3, $4, $5)`
	_, err := db.Exec(stmt, loc.DeliveryManID, loc.OrderId, loc.Latitude, loc.Longitude, loc.Timestamp)
	if err != nil {
		log.Println("Erro ao salvar no PostgreSQL:", err)
	}
}

func broadcastLocation(loc Location) {
	message, _ := json.Marshal(loc)

	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Println("Erro ao enviar via WebSocket:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro no upgrade para WebSocket:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()
	log.Println("Cliente conectado via WebSocket")

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
	log.Println("Cliente desconectado")
}

func handleIncomingLocation(w http.ResponseWriter, r *http.Request) {
	var loc Location
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	go saveLocationToDB(loc)
	go broadcastLocation(loc)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	initDB()

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/location", handleIncomingLocation)

	log.Println("Servidor escutando em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
