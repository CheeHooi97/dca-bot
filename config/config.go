package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	BinanceApiKey    string
	BinanceApiSecret string
	BTC2             string
	BTC1             string
	BTC1x5           string
	ETH2             string
	ETH1             string
	ETH1x5           string
	TelegramChatId   string
)

// LoadConfig
func LoadConfig() {
	_ = godotenv.Load()

	BinanceApiKey = GetEnv("BINANCE_API_KEY")
	BinanceApiSecret = GetEnv("BINANCE_API_SECRET")
	BTC2 = GetEnv("BTC2")
	BTC1 = GetEnv("BTC1")
	BTC1x5 = GetEnv("BTC1x5")
	ETH2 = GetEnv("ETH2")
	ETH1 = GetEnv("ETH1")
	ETH1x5 = GetEnv("ETH1x5")
	TelegramChatId = GetEnv("TELEGRAM_CHAT_ID")
}

func GetEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("%s environment variable not set", key)
	}
	return value
}
