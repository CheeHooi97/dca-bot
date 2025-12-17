package main

// import your bot package

// func main() {
// 	reader := bufio.NewReader(os.Stdin)

// 	// --- Prompt Symbol ---
// 	defaultSymbol := "ethusdt"
// 	fmt.Printf("Enter trading pair (default %s): ", defaultSymbol)
// 	symbolInput, _ := reader.ReadString('\n')
// 	symbol := strings.TrimSpace(symbolInput)
// 	if symbol == "" {
// 		symbol = defaultSymbol
// 	}

// 	// --- Prompt Total USDT ---
// 	defaultUSDT := 500.0
// 	fmt.Printf("Enter total USDT budget (default %.2f): ", defaultUSDT)
// 	usdtInput, _ := reader.ReadString('\n')
// 	usdtStr := strings.TrimSpace(usdtInput)
// 	totalUSDT := defaultUSDT
// 	if usdtStr != "" {
// 		if val, err := strconv.ParseFloat(usdtStr, 64); err == nil {
// 			totalUSDT = val
// 		} else {
// 			fmt.Println("Invalid input, using default.")
// 		}
// 	}

// 	// --- Prompt Stop Loss Percent ---
// 	defaultStopLoss := 1.5
// 	fmt.Printf("Enter Stop Loss percentage (default %.2f%%): ", defaultStopLoss)
// 	slInput, _ := reader.ReadString('\n')
// 	slStr := strings.TrimSpace(slInput)
// 	stopLossPercent := defaultStopLoss
// 	if slStr != "" {
// 		if val, err := strconv.ParseFloat(slStr, 64); err == nil {
// 			stopLossPercent = val
// 		} else {
// 			fmt.Println("Invalid input, using default.")
// 		}
// 	}

// 	// --- Prompt Low Boundary ---
// 	fmt.Printf("Enter Low Price boundary: ")
// 	lowInput, _ := reader.ReadString('\n')
// 	lowStr := strings.TrimSpace(lowInput)
// 	var lowPrice float64
// 	if lowStr != "" {
// 		if val, err := strconv.ParseFloat(lowStr, 64); err == nil {
// 			lowPrice = val
// 		} else {
// 			fmt.Println("Invalid input, exiting.")
// 			return
// 		}
// 	} else {
// 		fmt.Println("Low boundary required, exiting.")
// 		return
// 	}

// 	// --- Prompt High Boundary ---
// 	fmt.Printf("Enter High Price boundary: ")
// 	highInput, _ := reader.ReadString('\n')
// 	highStr := strings.TrimSpace(highInput)
// 	var highPrice float64
// 	if highStr != "" {
// 		if val, err := strconv.ParseFloat(highStr, 64); err == nil {
// 			highPrice = val
// 		} else {
// 			fmt.Println("Invalid input, exiting.")
// 			return
// 		}
// 	} else {
// 		fmt.Println("High boundary required, exiting.")
// 		return
// 	}

// 	// --- Initialize FixRangeBot ---
// 	botInstance := bot.NewFixRangeBot(symbol, totalUSDT)
// 	botInstance.StopLossPct = stopLossPercent / 100.0
// 	botInstance.LowPrice = lowPrice
// 	botInstance.HighPrice = highPrice
// 	botInstance.GridStep = (highPrice - lowPrice) / float64(botInstance.GridCount)

// 	// --- Start Bot with simulated price feed ---
// 	go func() {
// 		price := lowPrice
// 		increasing := true
// 		for {
// 			candle := bot.FixRangeCandle{
// 				High:  price + 1,
// 				Low:   price - 1,
// 				Close: price,
// 			}
// 			botInstance.OnPrice(symbol, price, candle)

// 			// Simulate price movement
// 			if increasing {
// 				price += botInstance.GridStep / 2
// 				if price >= highPrice {
// 					increasing = false
// 				}
// 			} else {
// 				price -= botInstance.GridStep / 2
// 				if price <= lowPrice {
// 					increasing = true
// 				}
// 			}

// 			time.Sleep(500 * time.Millisecond)
// 		}
// 	}()

// 	fmt.Println("FixRange bot started... (CTRL+C to exit)")
// 	select {} // keep the program alive
// }
