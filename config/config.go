package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	BinanceApiKey    string
	BinanceApiSecret string
	// BTC4h            string
	// BTC1d            string
	// ETH4h            string
	// ETH1d            string
	// ADA4h            string
	// ADA1d            string
	// SOL4h            string
	// SOL1d            string
	// BNB4h            string
	// XRP4h            string
	// DOGE4h           string
	// SUI4h            string
	// LINK4h           string
	// AVAX4h           string
	// TON4h            string
	// DOT4h            string
	// THETA4h          string
	// WLD4h            string
	// TIA4h            string
	// TRUMP4h          string
	TelegramChatId   string
	DCAToken         string
)

// LoadConfig
func LoadConfig() {
	_ = godotenv.Load()

	BinanceApiKey = GetEnv("BINANCE_API_KEY")
	BinanceApiSecret = GetEnv("BINANCE_API_SECRET")
	// BTC4h = GetEnv("BTC_4h")
	// BTC1d = GetEnv("BTC_1d")
	// ETH4h = GetEnv("ETH_4h")
	// ETH1d = GetEnv("ETH_1d")
	// ADA4h = GetEnv("ADA_4h")
	// ADA1d = GetEnv("ADA_1d")
	// SOL4h = GetEnv("SOL_4h")
	// SOL1d = GetEnv("SOL_1d")
	// BNB4h = GetEnv("BNB_4h")
	// XRP4h = GetEnv("XRP_4h")
	// DOGE4h = GetEnv("DOGE_4h")
	// SUI4h = GetEnv("SUI_4h")
	// LINK4h = GetEnv("LINK_4h")
	// AVAX4h = GetEnv("AVAX_4h")
	// TON4h = GetEnv("TON_4h")
	// DOT4h = GetEnv("DOT_4h")
	// THETA4h = GetEnv("THETA_4h")
	// WLD4h = GetEnv("WLD_4h")
	// TIA4h = GetEnv("TIA_4h")
	// TRUMP4h = GetEnv("TRUMP_4h")
	TelegramChatId = GetEnv("TELEGRAM_CHAT_ID")
	DCAToken = GetEnv("DCA_TOKEN")
}

func GetEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("%s environment variable not set", key)
	}
	return value
}
