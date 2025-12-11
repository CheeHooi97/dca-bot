package handler

import (
	"bufio"
	"dca-bot/service"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type DCAHandler struct {
	service *service.DCAService
}

func NewDCAHandler() *DCAHandler {
	return &DCAHandler{
		service: service.NewDCAService(),
	}
}

func (h *DCAHandler) StartDCA() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Enter trading pair (e.g. btcusdt): ")
	symbol, _ := reader.ReadString('\n')
	symbol = strings.TrimSpace(symbol)

	fmt.Printf("Enter total USDT amount for DCA: ")
	usdtStr, _ := reader.ReadString('\n')
	usdtStr = strings.TrimSpace(usdtStr)
	totalUsdt, err := strconv.ParseFloat(usdtStr, 64)
	if err != nil {
		return fmt.Errorf("invalid USDT amount")
	}

	fmt.Printf("Enter drop percentage to trigger buy (e.g. 2): ")
	dropStr, _ := reader.ReadString('\n')
	dropStr = strings.TrimSpace(dropStr)
	dropPercent, err := strconv.ParseFloat(dropStr, 64)
	if err != nil {
		return fmt.Errorf("invalid drop percentage")
	}

	fmt.Print("Fallback Buy Again: ")
	fallbackBuyHoursInput, _ := reader.ReadString('\n')
	fallbackBuyHoursStr := strings.TrimSpace(fallbackBuyHoursInput)

	fallbackBuyHours, _ := strconv.ParseInt(fallbackBuyHoursStr, 10, 64)

	return h.service.Start(symbol, totalUsdt, dropPercent, int(fallbackBuyHours))
}
