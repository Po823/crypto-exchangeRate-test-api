package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)
 type ExchangeRate struct {
	Crypto    string    `json:"crypto"`
	Fiat      string    `json:"fiat"`
	Rate      float64   `json:"rate"`
	Timestamp time.Time `json:"timestamp"`
}
 var db *sql.DB
 func main() {
	// Establish a connection to the MySQL database
	var err error
	db, err = sql.Open("mysql", "root:@tcp(localhost:3306)/crypto-api")
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}
	defer db.Close()
 	// Create the exchange_rates table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS exchange_rates (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT,
		crypto VARCHAR(50) NOT NULL,
		fiat VARCHAR(50) NOT NULL,
		rate FLOAT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		PRIMARY KEY (id)

	)`)
	if err != nil {
		fmt.Println("Error creating table:", err)
		return
	}
 	// Run the initial update
	updateExchangeRates()
 	// Schedule periodic updates every 5 minutes
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				updateExchangeRates()
			}
		}
	}()
 	// Create a new router
	router := mux.NewRouter()
	// Define the routes
	router.HandleFunc("/rates", getAllExchangeRatesHandler).Methods("GET")
	router.HandleFunc("/rates/{cryptocurrency}/{fiat}", getExchangeRateHandler).Methods("GET")
	router.HandleFunc("/rates/{cryptocurrency}", getCryptoExchangeRatesHandler).Methods("GET")
	router.HandleFunc("/rates/history/{cryptocurrency}/{fiat}", getExchangeRateHistoryHandler).Methods("GET")
	router.HandleFunc("/balance/{address}", getBalance).Methods("GET")
	// Start the server
	log.Println("Server listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
 func updateExchangeRates() {
	// Get the latest exchange rates
	rates, err := getExchangeRates()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
 	// Store the rates in the database
	err = storeExchangeRates(rates)
	if err != nil {
		fmt.Println("Error storing exchange rates:", err)
		return
	}
 	fmt.Println("Exchange rates updated successfully")
}
 func getExchangeRates() ([]ExchangeRate, error) {
	// Get a list of all supported fiat currencies
	fiatCurrencies, err := getSupportedFiatCurrencies()
	if err != nil {
		return nil, err
	}
 	// Get a list of all supported cryptocurrencies
	cryptoCurrencies, err := getSupportedCryptoCurrencies()
	if err != nil {
		return nil, err
	}
 	// Make a request to the CoinGecko API for exchange rates
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s", strings.Join(cryptoCurrencies, ","), strings.Join(fiatCurrencies, ","))
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
 	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
 	// Parse the response JSON
	var rates map[string]map[string]float64
	err = json.Unmarshal(body, &rates)
	if err != nil {
		return nil, err
	}
 	// Convert the response to ExchangeRate struct
	var exchangeRates []ExchangeRate
	for crypto, fiatRates := range rates {
		for fiat, rate := range fiatRates {
			exchangeRate := ExchangeRate{
				Crypto:    crypto,
				Fiat:      fiat,
				Rate:      rate,
				Timestamp: time.Now(),
			}
			exchangeRates = append(exchangeRates, exchangeRate)
		}
	}
 	return exchangeRates, nil
}
 func getSupportedFiatCurrencies() ([]string, error) {
	// Make a request to the CoinGecko API for supported fiat currencies
	url := "https://api.coingecko.com/api/v3/simple/supported_vs_currencies"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
 	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
 	// Parse the response JSON
	var fiatCurrencies []string
	err = json.Unmarshal(body, &fiatCurrencies)
	if err != nil {
		return nil, err
	}
 	return fiatCurrencies, nil
	 return []string{"usd", "eur", "gbp"}, nil
}
 func getSupportedCryptoCurrencies() ([]string, error) {
	// Make a request to the CoinGecko API for supported cryptocurrencies
	url := "https://api.coingecko.com/api/v3/coins/list"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
 	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
 	// Parse the response JSON
	var cryptoCurrencies []struct {
		ID string `json:"id"`
	}
	err = json.Unmarshal(body, &cryptoCurrencies)
	if err != nil {
		return nil, err
	}
 	var cryptoCurrencyIDs []string
	for _, crypto := range cryptoCurrencies {
		cryptoCurrencyIDs = append(cryptoCurrencyIDs, crypto.ID)
	}
 	// return cryptoCurrencyIDs, nil
	 return []string{"bitcoin", "ethereum", "litecoin"}, nil
}
 func storeExchangeRates(rates []ExchangeRate) error {
	// Prepare the SQL statement
	
	stmt, err := db.Prepare("INSERT INTO exchange_rates (crypto, fiat, rate, timestamp) VALUES (?, ?, ?, ?) ")
	if err != nil {
		return err
	}
	defer stmt.Close()
 	// Insert or update each rate in the database
	for _, rate := range rates {
		_, err = stmt.Exec(rate.Crypto, rate.Fiat, rate.Rate, rate.Timestamp)
		if err != nil {
			return err
		}
	}
 	return nil
}
 func getAllExchangeRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve all exchange rates from the database
	query := `SELECT crypto, fiat, rate, timestamp FROM exchange_rates 
			    WHERE  timestamp = (
					SELECT MAX(timestamp)
					FROM exchange_rates
				)`
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
 	var exchangeRates []ExchangeRate
	for rows.Next() {
		var crypto, fiat string
		var rate float64
		// var timestamp time.Time
		var timestampStr string
		err := rows.Scan(&crypto, &fiat, &rate, &timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
 		exchangeRate := ExchangeRate{
			Crypto:    crypto,
			Fiat:      fiat,
			Rate:      rate,
			Timestamp: timestamp,
		}
		exchangeRates = append(exchangeRates, exchangeRate)
	}
 	json.NewEncoder(w).Encode(exchangeRates)
}
 func getExchangeRateHandler(w http.ResponseWriter, r *http.Request) {
	// Extract cryptocurrency and fiat parameters from the URL
	params := mux.Vars(r)
	crypto := params["cryptocurrency"]
	fiat := params["fiat"]

	// Retrieve the exchange rate from the database
	query := `SELECT rate, timestamp FROM exchange_rates WHERE crypto = ? AND fiat = ?`
	row := db.QueryRow(query, crypto, fiat)

	var rate float64
	var timestampStr string
	err := row.Scan(&rate, &timestampStr)
	if err != nil {
		if err == sql.ErrNoRows {
			// Exchange rate not found in the database, retrieve it from the CoinGecko API
			exchangeRate, err := getExchangeRateFromAPI(crypto, fiat)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Store the exchange rate in the database
			query = `INSERT INTO exchange_rates (crypto, fiat, rate, timestamp) VALUES (?, ?, ?, NOW())`
			_, err = db.Exec(query, crypto, fiat, exchangeRate)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Return the exchange rate as JSON
			exchangeRateData := ExchangeRate{
				Crypto:    crypto,
				Fiat:      fiat,
				Rate:      exchangeRate,
				Timestamp: time.Now(),
			}
			json.NewEncoder(w).Encode(exchangeRateData)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	exchangeRate := ExchangeRate{
		Crypto:    crypto,
		Fiat:      fiat,
		Rate:      rate,
		Timestamp: timestamp,
	}

	json.NewEncoder(w).Encode(exchangeRate)
}
func getExchangeRateFromAPI(crypto, fiat string) (float64, error) {
	// Construct the URL to fetch the exchange rate from the CoinGecko API
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s", crypto, fiat)

	// Make a GET request to the CoinGecko API
	resp, err := http.Get(url)
	if err != nil {
		return 0.0, err
	}
	defer resp.Body.Close()

	// Parse the JSON response to extract the exchange rate
	var data map[string]map[string]float64
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return 0.0, err
	}

	rates, ok := data[crypto]
	if !ok {
		return 0.0, fmt.Errorf("Exchange rate not found for %s/%s", crypto, fiat)
	}

	exchangeRate, ok := rates[fiat]
	if !ok {
		return 0.0, fmt.Errorf("Exchange rate not found for %s/%s", crypto, fiat)
	}

	return exchangeRate, nil
}
 func getCryptoExchangeRatesHandler(w http.ResponseWriter, r *http.Request) {
	// Extract cryptocurrency parameter from the URL
	params := mux.Vars(r)
	crypto := params["cryptocurrency"]
 	// Retrieve the latest exchange rates for the specified cryptocurrency from the database
	query := `
		SELECT fiat, rate, timestamp
		FROM exchange_rates
		WHERE crypto = ?
		AND timestamp = (
			SELECT MAX(timestamp)
			FROM exchange_rates
			WHERE crypto = ?
		)
	`
	rows, err := db.Query(query, crypto, crypto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
 	var exchangeRates []ExchangeRate
	for rows.Next() {
		var fiat string
		var rate float64
		var timestampStr string
		err := rows.Scan(&fiat, &rate, &timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
 		exchangeRate := ExchangeRate{
			Crypto:    crypto,
			Fiat:      fiat,
			Rate:      rate,
			Timestamp: timestamp,
		}
		exchangeRates = append(exchangeRates, exchangeRate)
	}
 	json.NewEncoder(w).Encode(exchangeRates)
}

 func getExchangeRateHistoryHandler(w http.ResponseWriter, r *http.Request) {
	// Extract cryptocurrency and fiat currency parameters from the URL
	params := mux.Vars(r)
	crypto := params["cryptocurrency"]
	fiat := params["fiat"]

	// Calculate the start and end timestamps for the past 24 hours
	endTime := time.Now().UTC()
	startTime := endTime.Add(-24 * time.Hour)

	// Retrieve the exchange rate history for the specified cryptocurrency and fiat currency from the database
	query := `SELECT rate, timestamp FROM exchange_rates WHERE crypto = ? AND fiat = ? AND timestamp >= ? AND timestamp <= ?`
	rows, err := db.Query(query, crypto, fiat, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var exchangeRateHistory []ExchangeRate
	for rows.Next() {
		var rate float64
		var timestampStr string
		err := rows.Scan(&rate, &timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		exchangeRate := ExchangeRate{
			Crypto:    crypto,
			Fiat:      fiat,
			Rate:      rate,
			Timestamp: timestamp,
		}
		exchangeRateHistory = append(exchangeRateHistory, exchangeRate)
	}

	json.NewEncoder(w).Encode(exchangeRateHistory)
}

 func getBalance(w http.ResponseWriter, r *http.Request) {
	address := mux.Vars(r)["address"]

	client, err := ethclient.Dial("https://mainnet.infura.io/v3/f39b5325b7984894a7c968c3dd37aef0")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account := common.HexToAddress(address)
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balanceInEther := weiToEther(balance)

	response := struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
	}{
		Address: address,
		Balance: balanceInEther.String(),
	}

	json.NewEncoder(w).Encode(response)
}

func weiToEther(wei *big.Int) *big.Float {
	ether := new(big.Float)
	ether.SetString(wei.String())
	ether = ether.Quo(ether, big.NewFloat(math.Pow10(18)))
	return ether
}