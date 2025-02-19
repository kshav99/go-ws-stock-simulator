# go-ws-stock-simulator
Quick proof-of-concept demonstrating real-time stock data via WebSockets in GO using simulated data.
## Description

This project provides a simple WebSocket server written in Go that simulates real-time stock price updates. Clients can connect via WebSockets to receive live stock data, set alerts for specific stocks, and view initial stock data via a REST API.

## Features

* **Real-time Stock Simulation:** Simulates stock price fluctuations for a predefined list of symbols.
* **WebSocket Communication:** Clients receive live stock updates through WebSockets.
* **Alert System:** Clients can set alerts for specific stock prices (above or below a threshold).
* **Initial Stock Data API:** A REST API endpoint (`/api/stocks`) provides initial stock data in JSON format.
* **CORS Enabled:** The server is configured to allow cross-origin requests.

## Getting Started

### Prerequisites

* Go (version 1.16 or later)

### Installation

1.  Clone the repository:

    ```bash
    git clone <repository_url>
    cd live-stock-sim-ws
    ```

2.  Initialize Go modules:

    ```bash
    go mod init live-stock-sim-ws
    go mod tidy
    ```

### Running the Server

1.  Start the Go server:

    ```bash
    go run main.go
    ```

    The server will start listening on port 8080.
