package entity

import "gorm.io/gorm"

type CartItem struct {
	gorm.Model
	CartID      uint
	ProductName string
	Quantity    int
	Price       float64
}
