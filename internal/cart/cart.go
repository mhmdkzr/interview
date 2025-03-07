package cart

import "gorm.io/gorm"

const (
	StatusOpen   = "open"
	StatusClosed = "closed"
)

type (
	Cart struct {
		gorm.Model
		SessionID string `gorm:"size:255;uniqueIndex;not null"`
		Status    string `gorm:"size:64;index;not null"`
		Total     float64
		CartItems []CartItem
	}

	CartItem struct {
		gorm.Model
		CartID      uint `gorm:"index;not null"`
		ProductName string
		Quantity    int
		Price       float64
	}
)
