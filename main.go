package main

import (
	"bufio"
	"dca-bot/config"
	"dca-bot/service"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	// Load config
	config.LoadConfig()

	reader := bufio.NewReader(os.Stdin)

	// --- Input: Symbol ---
	fmt.Print("Enter trading pair (e.g. btcusdt): ")
	symbolInput, _ := reader.ReadString('\n')
	symbol := strings.TrimSpace(symbolInput)

	// --- Input: Total USDT for DCA ---
	fmt.Print("Enter total USDT budget for DCA: ")
	usdtInput, _ := reader.ReadString('\n')
	usdtStr := strings.TrimSpace(usdtInput)

	totalUSDT, err := strconv.ParseFloat(usdtStr, 64)
	if err != nil {
		fmt.Println("Invalid USDT amount")
		return
	}

	// --- Input: Drop percent trigger ---
	fmt.Print("Enter drop percentage trigger (e.g. 1.5): ")
	dropInput, _ := reader.ReadString('\n')
	dropStr := strings.TrimSpace(dropInput)

	dropPercent, err := strconv.ParseFloat(dropStr, 64)
	if err != nil {
		fmt.Println("Invalid drop percent")
		return
	}

	fmt.Print("Fallback Buy Again: ")
	fallbackBuyHoursInput, _ := reader.ReadString('\n')
	fallbackBuyHoursStr := strings.TrimSpace(fallbackBuyHoursInput)

	fallbackBuyHours, _ := strconv.ParseInt(fallbackBuyHoursStr, 10, 64)

	fmt.Print("Enter sell percentage(e.g. 1.5): ")
	sellInput, _ := reader.ReadString('\n')
	sellStr := strings.TrimSpace(sellInput)

	sellPercent, err := strconv.ParseFloat(sellStr, 64)
	if err != nil {
		fmt.Println("Invalid drop percent")
		return
	}

	// Initialize service
	dcaService := service.NewDCAService()

	// Start DCA bot
	err = dcaService.Start(symbol, totalUSDT, dropPercent, sellPercent, int(fallbackBuyHours))
	if err != nil {
		fmt.Println("Error starting DCA:", err)
		return
	}

	// Keep the program alive so goroutine runs
	fmt.Println("DCA bot started... (CTRL+C to exit)")
	select {}
}
