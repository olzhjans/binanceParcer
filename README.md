# Binance Ticker Parser
This project is a Binance Ticker Parser written in Go. It fetches the latest prices for a list of symbols from the Binance API, distributes the symbols across multiple workers, and prints the prices to the standard output.

## Functionality
- Fetches real-time price data from the Binance API
- Distributes work across multiple workers
- Prints price changes to the standard output
- Monitors and prints the total number of requests made by workers each 5 seconds

## Requirements
- Golang 1.15 or higher

## Installation
1. Clone the repository:
   ```bash
   git clone https://github.com/olzhjans/binanceParcer
   ```
2. Install dependencies:
   ```bash
   go mod tidy
   ```

## Run program
   ```bash
   go run main.go
   ```
The program will start fetching prices for the specified symbols from `config.yaml` file.<br>
To stop the program, type `STOP` and press Enter.