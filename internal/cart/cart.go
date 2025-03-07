package cart

import "gorm.io/gorm"

const (
	// StatusOpen represents an active shopping cart that can be modified
	StatusOpen = "open"
	// StatusClosed represents a completed shopping cart that can no longer be modified
	StatusClosed = "closed"
)

type (
	// Cart represents a shopping cart associated with a user session
	Cart struct {
		gorm.Model
		// SessionID uniquely identifies the user's session
		SessionID string `gorm:"size:255;uniqueIndex;not null"`
		// Status indicates whether the cart is open or closed
		Status string `gorm:"size:64;index;not null"`
		// Total represents the total price of all items in the cart
		Total float64
		// CartItems contains all items added to the cart
		CartItems []CartItem
	}

	// CartItem represents a single item in the shopping cart
	CartItem struct {
		gorm.Model
		// CartID links the item to its parent cart
		CartID uint `gorm:"index;not null"`
		// ProductName is the name of the product
		ProductName string
		// Quantity represents the number of items ordered
		Quantity int
		// Price represents the unit price of the item
		Price float64
	}
)
