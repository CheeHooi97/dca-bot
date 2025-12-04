package repository

import "fmt"

type DCARepository struct{}

func NewDCARepository() *DCARepository {
	return &DCARepository{}
}

func (r *DCARepository) Save(symbol string, totalUsdt, dropPercent float64) {
	fmt.Printf("[repo] DCA session saved — %s | total %.2f | drop %.2f%%\n",
		symbol, totalUsdt, dropPercent)
}
