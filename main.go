package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"dca-bot/config"
	"dca-bot/service" // Update with your actual package path

	bybit "github.com/bybit-exchange/bybit.go.api"
)

func main() {
	// 1. Load config
	config.LoadConfig()

	reader := bufio.NewReader(os.Stdin)

	// 2. Input: Symbol
	fmt.Print("Enter trading pair (e.g. BTCUSDT): ")
	symbolInput, _ := reader.ReadString('\n')
	symbol := strings.TrimSpace(strings.ToUpper(symbolInput))

	fmt.Printf("DEBUG: Key Length: %d, Secret Length: %d\n", len(config.BybitApiKey), len(config.BybitApiSecret))
	if config.BybitApiKey == "" || config.BybitApiSecret == "" {
		log.Fatal("API Key or Secret is empty! Check your config loading.")
	}

	// 3. Initialize Bybit Client
	client := bybit.NewBybitHttpClient(config.BybitApiKey, config.BybitApiSecret, bybit.WithBaseURL(bybit.MAINNET))

	serverTime, err := client.NewUtaBybitServiceNoParams().GetServerTime(context.Background())
	if err != nil {
		fmt.Println("Network Error: Cannot even reach Bybit!", err)
	} else {
		fmt.Println("Network OK. Server Time:", serverTime.Result)
	}

	// 4. Fetch Wallet Balance from Bybit
	params := map[string]interface{}{
		"accountType": "UNIFIED", // Options: UNIFIED, SPOT, CONTRACT
		"coin":        "USDT",
	}

	// Use GetAccountWallet instead of GetWalletBalance
	res, err := client.NewUtaBybitServiceWithParams(params).GetAccountWallet(context.Background())
	if err != nil {
		// This will print the actual network or validation error
		log.Fatalf("Critical Connection Error: %v", err)
	}

	if res == nil {
		log.Fatal("Bybit returned a nil response without an error. Check your internet or API URL.")
	}

	var balance float64

	switch v := res.Result.(type) {
	case []byte:
		// Case 1: Result is raw JSON (needs Unmarshal)
		var data struct {
			List []struct {
				Coin []struct {
					Coin          string `json:"coin"`
					WalletBalance string `json:"walletBalance"`
				} `json:"coin"`
			} `json:"list"`
		}
		if err := json.Unmarshal(v, &data); err == nil && len(data.List) > 0 {
			for _, c := range data.List[0].Coin {
				if c.Coin == "USDT" {
					balance, _ = strconv.ParseFloat(c.WalletBalance, 64)
					break
				}
			}
		}

	case map[string]interface{}:
		// Case 2: Result is already a map (needs type assertions)
		if list, ok := v["list"].([]interface{}); ok && len(list) > 0 {
			if account, ok := list[0].(map[string]interface{}); ok {
				if coins, ok := account["coin"].([]interface{}); ok {
					for _, c := range coins {
						if coinData, ok := c.(map[string]interface{}); ok {
							if coinData["coin"] == "USDT" {
								balanceStr, _ := coinData["walletBalance"].(string)
								balance, _ = strconv.ParseFloat(balanceStr, 64)
								break
							}
						}
					}
				}
			}
		}
	default:
		fmt.Printf("Unknown result type: %T\n", v)
	}

	if balance <= 0 {
		fmt.Println("❌ Could not retrieve USDT balance. Please check if funds are in your Unified/Spot account.")
		return
	}
	fmt.Printf("✅ Balance found: %.2f USDT\n", balance)

	// 6. Input: Parameters
	fmt.Print("Enter drop percentage trigger (e.g. 1.5): ")
	dropInput, _ := reader.ReadString('\n')
	dropPercent, _ := strconv.ParseFloat(strings.TrimSpace(dropInput), 64)

	fmt.Print("Fallback Buy Hours (e.g. 24): ")
	fbInput, _ := reader.ReadString('\n')
	fallbackBuyHours, _ := strconv.ParseInt(strings.TrimSpace(fbInput), 10, 64)

	fmt.Print("Enter sell percentage (e.g. 1.5): ")
	sellInput, _ := reader.ReadString('\n')
	sellPercent, _ := strconv.ParseFloat(strings.TrimSpace(sellInput), 64)

	// 7. Initialize and Start Service
	dcaService := service.NewDCAService()
	err = dcaService.Start(client, symbol, balance, dropPercent, sellPercent, int(fallbackBuyHours))
	if err != nil {
		fmt.Println("Error starting DCA:", err)
		return
	}

	fmt.Println("🚀 DCA bot is now running... (CTRL+C to exit)")
	select {}
}

func processResultMap(resultMap map[string]interface{}, balance *float64) {
	if list, ok := resultMap["list"].([]interface{}); ok && len(list) > 0 {
		account := list[0].(map[string]interface{})
		if coins, ok := account["coin"].([]interface{}); ok {
			for _, c := range coins {
				coinData := c.(map[string]interface{})
				if coinData["coin"] == "USDT" {
					val, _ := strconv.ParseFloat(coinData["walletBalance"].(string), 64)
					*balance = val
					return
				}
			}
		}
	}
}
