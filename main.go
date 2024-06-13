package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Config struct {
	Symbols    []string `yaml:"symbols"`
	MaxWorkers int      `yaml:"max_workers"`
}

type PriceResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Worker struct {
	symbols       []string
	requestCount  int // не закрыл мьютексом, хотя этот счетчик тоже в разных горутинах юзается
	priceCache    map[string]string
	priceCacheMux sync.Mutex // хорошая прктика писать под мьютексом, все что мы им закрываем
}

func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()
	client := &http.Client{} // если спецом или случайно запустим несколько раз метод Run то будет алоцировать новый клиент кажд раз и еще если клиент без таймаунта, то он может зависнуть навсегда
	for {
		select {
		case <-ctx.Done():
			return
		default:
			for _, symbol := range w.symbols { // селект лучше в этот цикл, иначе чтобы остановиться надо ждать когда по всем символам пройдемся
				url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%s", symbol)
				resp, err := client.Get(url)
				if err != nil {
					log.Println("Error fetching price:", err)
					continue
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Println("Error reading response body:", err)
					err = resp.Body.Close() // лучше через defer
					if err != nil {
						return
					}
					continue
				}
				err = resp.Body.Close()
				if err != nil {
					return
				}

				var priceResp PriceResponse
				if err = json.Unmarshal(body, &priceResp); err != nil {
					log.Println("Error unmarshalling response:", err)
					continue
				}

				w.priceCacheMux.Lock()
				oldPrice, exists := w.priceCache[priceResp.Symbol]
				w.priceCache[priceResp.Symbol] = priceResp.Price
				w.priceCacheMux.Unlock()

				message := fmt.Sprintf("%s price:%s", priceResp.Symbol, priceResp.Price)
				if exists && oldPrice != priceResp.Price {
					message += " changed" // так тоже можно, но я бы разделил логику сбора цен и логику определения changed
				}
				results <- message

				w.requestCount++ // счетчик должен учитывать и ошибочные запросы
			}
		}
	}
}

func (w *Worker) GetRequestsCount() int {
	return w.requestCount
}

func main() {
	// Read config file
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	var config Config
	if err = yaml.Unmarshal(configData, &config); err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	// Adjust max workers
	numCPU := 12 // ??? у меня на машине не 12 ядер
	if config.MaxWorkers > numCPU {
		config.MaxWorkers = numCPU
	}

	// Распределение символов по воркерам
	workers := make([]*Worker, config.MaxWorkers) // если символов меньше чем воркеров, создаим лишних воркеров
	for i := 0; i < config.MaxWorkers; i++ {
		workers[i] = &Worker{
			symbols:    []string{},
			priceCache: make(map[string]string),
		}
	}
	for i, symbol := range config.Symbols {
		workers[i%config.MaxWorkers].symbols = append(workers[i%config.MaxWorkers].symbols, symbol) // если MaxWorkers=0 словишь панику
	}

	// Run workers
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	results := make(chan string)

	for _, worker := range workers {
		wg.Add(1)
		go worker.Run(ctx, &wg, results)
	}

	// Подсчет количества запросов
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				totalRequests := 0
				for _, worker := range workers {
					totalRequests += worker.GetRequestsCount()
				}
				fmt.Printf("workers requests total: %d\n", totalRequests)
			}
		}
	}()

	// Handle results
	go func() {
		for result := range results {
			fmt.Println(result)
		}
	}()

	// Команда "STOP"
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if scanner.Text() == "STOP" {
			cancel()
			break
		}
	}

	// Wait for workers to finish
	wg.Wait()
	close(results)
}
