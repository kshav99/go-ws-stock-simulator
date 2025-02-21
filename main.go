package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// enableCors sets basic CORS headers.
func enableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type StockPrice struct {
	Symbol        string    `json:"symbol"`
	Price         float32   `json:"price"`
	Change        float32   `json:"change"`
	ChangePercent float32   `json:"changePercent"`
	Volume        int64     `json:"volume"`
	High          float32   `json:"high"`
	Low           float32   `json:"low"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type StockAlert struct {
	Symbol    string  `json:"symbol"`
	Condition string  `json:"condition"` // "above" or "below"
	Price     float32 `json:"price"`
}

var (
	clients    = make(map[*websocket.Conn]bool)
	broadcast  = make(chan StockPrice)
	alerts     = make(map[*websocket.Conn][]StockAlert)
	stocksData = make(map[string]*StockPrice)
	mutex      sync.RWMutex
)

func main() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/stocks", getStocksHandler)
	http.HandleFunc("/health", healthHandler)

	go simulateStockUpdates()
	go handleMessages()

	handler := enableCors(http.DefaultServeMux)

	log.Println("[Server] HTTP server started on port :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// healthHandler provides a simple endpoint for health checks.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// getStocksHandler returns the current stocks data as JSON.
func getStocksHandler(w http.ResponseWriter, r *http.Request) {
	mutex.RLock()
	stockList := make([]StockPrice, 0, len(stocksData))
	for _, stock := range stocksData {
		stockList = append(stockList, *stock)
	}
	mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stockList)
}

// handleConnections upgrades HTTP requests to WebSocket connections and processes messages.
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Server] Upgrade error: %v", err)
		return
	}
	defer ws.Close()

	clients[ws] = true
	alerts[ws] = make([]StockAlert, 0)
	log.Printf("[Server] New WebSocket connection: %s", ws.RemoteAddr())

	// Send initial stock data to the newly connected client.
	mutex.RLock()
	initialStocks := make([]StockPrice, 0, len(stocksData))
	for _, stock := range stocksData {
		initialStocks = append(initialStocks, *stock)
	}
	mutex.RUnlock()
	if err := ws.WriteJSON(initialStocks); err != nil {
		log.Printf("[Server] Error sending initial stocks: %v", err)
		removeClient(ws)
		return
	}

	// Listen for messages from client.
	for {
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("[Server] ReadJSON error: %v", err)
			removeClient(ws)
			break
		}

		switch msg.Type {
		case "SET_ALERT":
			var alert StockAlert
			if err := json.Unmarshal(msg.Payload, &alert); err != nil {
				log.Printf("[Server] Error parsing SET_ALERT payload: %v", err)
				continue
			}
			alerts[ws] = append(alerts[ws], alert)
			log.Printf("[Server] Alert set for %s: %s %.2f", alert.Symbol, alert.Condition, alert.Price)
		case "REMOVE_ALERT":
			var alert StockAlert
			if err := json.Unmarshal(msg.Payload, &alert); err != nil {
				log.Printf("[Server] Error parsing REMOVE_ALERT payload: %v", err)
				continue
			}
			// Remove the first matching alert.
			updatedAlerts := []StockAlert{}
			removed := false
			for _, a := range alerts[ws] {
				if !removed && a.Symbol == alert.Symbol && a.Condition == alert.Condition && a.Price == alert.Price {
					removed = true
					continue
				}
				updatedAlerts = append(updatedAlerts, a)
			}
			alerts[ws] = updatedAlerts
			log.Printf("[Server] Alert removed for %s: %s %.2f", alert.Symbol, alert.Condition, alert.Price)
		default:
			log.Printf("[Server] Unknown message type: %s", msg.Type)
		}
	}
}

// removeClient cleans up a disconnected client.
func removeClient(ws *websocket.Conn) {
	ws.Close()
	delete(clients, ws)
	delete(alerts, ws)
}

// handleMessages listens for stock updates and broadcasts them to connected clients.
func handleMessages() {
	for {
		stock := <-broadcast
		checkAlerts(stock)

		for client := range clients {
			if err := client.WriteJSON(stock); err != nil {
				log.Printf("[Server] WriteJSON error: %v", err)
				removeClient(client)
			}
		}
	}
}

// checkAlerts sends an alert message to clients if their set conditions are met.
func checkAlerts(stock StockPrice) {
	for client, clientAlerts := range alerts {
		for _, alert := range clientAlerts {
			if alert.Symbol == stock.Symbol {
				shouldAlert := (alert.Condition == "above" && stock.Price > alert.Price) ||
					(alert.Condition == "below" && stock.Price < alert.Price)
				if shouldAlert {
					alertMsg := struct {
						Type    string     `json:"type"`
						Message string     `json:"message"`
						Stock   StockPrice `json:"stock"`
					}{
						Type:    "ALERT",
						Message: fmt.Sprintf("%s price is %s %.2f", stock.Symbol, alert.Condition, alert.Price),
						Stock:   stock,
					}
					if err := client.WriteJSON(alertMsg); err != nil {
						log.Printf("[Server] Error sending alert to client: %v", err)
						removeClient(client)
					}
				}
			}
		}
	}
}

// simulateStockUpdates generates periodic stock updates.
func simulateStockUpdates() {
	symbols := []string{"AAPL", "GOOGL", "MSFT", "AMZN", "TSLA", "META", "NVDA", "AMD", "INTC", "NFLX"}
	baselinePrices := map[string]float32{
		"AAPL": 175.0, "GOOGL": 140.0, "MSFT": 380.0, "AMZN": 175.0, "TSLA": 190.0,
		"META": 485.0, "NVDA": 850.0, "AMD": 180.0, "INTC": 43.0, "NFLX": 600.0,
	}

	// Initialize stocks data.
	for _, symbol := range symbols {
		basePrice := baselinePrices[symbol]
		stocksData[symbol] = &StockPrice{
			Symbol:    symbol,
			Price:     basePrice,
			High:      basePrice,
			Low:       basePrice,
			Volume:    0,
			UpdatedAt: time.Now(),
		}
	}

	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		for _, stock := range stocksData {
			mutex.Lock()
			// Simulate realistic price movements.
			change := (rand.Float32() - 0.5) * (stock.Price * 0.02)
			newPrice := stock.Price + change

			stock.Change = change
			stock.Price = newPrice
			stock.ChangePercent = (change / stock.Price) * 100
			stock.Volume += rand.Int63n(10000)
			stock.UpdatedAt = time.Now()

			if newPrice > stock.High {
				stock.High = newPrice
			}
			if newPrice < stock.Low {
				stock.Low = newPrice
			}
			mutex.Unlock()

			broadcast <- *stock
		}
	}
}
