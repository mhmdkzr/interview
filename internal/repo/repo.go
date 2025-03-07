package repo

import (
	"errors"
	"fmt"
	cartpkg "interview/internal/cart"
	"interview/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// InitDatabase initializes the MySQL database connection and performs auto-migration
func InitDatabase(config config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.DBUser,
		config.DBPassword,
		config.DBHost,
		config.DBPort,
		config.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	if err := db.AutoMigrate(&cartpkg.Cart{}, &cartpkg.CartItem{}); err != nil {
		return nil, fmt.Errorf("database migration failed: %w", err)
	}

	return db, nil
}

func (r *Repository) GetOrCreateCart(sessionID string) (*cartpkg.Cart, error) {
	var userCart cartpkg.Cart

	// Only consider open carts
	err := r.db.Preload("CartItems").
		Where("session_id = ? AND status = ?", sessionID, cartpkg.StatusOpen).
		First(&userCart).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		userCart = cartpkg.Cart{
			SessionID: sessionID,
			Status:    cartpkg.StatusOpen,
		}
		if err := r.db.Create(&userCart).Error; err != nil {
			return nil, fmt.Errorf("failed to create new cart: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get cart: %w", err)
	}

	return &userCart, nil
}

func (r *Repository) AddCartItem(cartID uint, productName string, quantity int, price float64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var cart cartpkg.Cart
		if err := tx.First(&cart, cartID).Error; err != nil {
			return fmt.Errorf("cart not found: %w", err)
		}

		if cart.Status != cartpkg.StatusOpen {
			return errors.New("cannot add items to a closed cart")
		}

		var existingItem cartpkg.CartItem
		err := tx.Where("cart_id = ? AND product_name = ?", cartID, productName).
			First(&existingItem).Error

		if err == nil {
			existingItem.Quantity += quantity
			if err := tx.Save(&existingItem).Error; err != nil {
				return fmt.Errorf("failed to update item: %w", err)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			item := cartpkg.CartItem{
				CartID:      cartID,
				ProductName: productName,
				Quantity:    quantity,
				Price:       price,
			}
			if err := tx.Create(&item).Error; err != nil {
				return fmt.Errorf("failed to create item: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check items: %w", err)
		}

		return r.updateCartTotal(tx, cartID)
	})
}

func (r *Repository) RemoveCartItem(cartID uint, itemID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var cart cartpkg.Cart
		if err := tx.First(&cart, cartID).Error; err != nil {
			return fmt.Errorf("cart not found: %w", err)
		}

		if cart.Status != cartpkg.StatusOpen {
			return errors.New("cannot remove items from a closed cart")
		}

		var item cartpkg.CartItem
		if err := tx.Where("cart_id = ? AND id = ?", cartID, itemID).
			First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("item not found")
			}
			return fmt.Errorf("failed to find item: %w", err)
		}

		if err := tx.Delete(&item).Error; err != nil {
			return fmt.Errorf("failed to remove item: %w", err)
		}

		return r.updateCartTotal(tx, cartID)
	})
}

func (r *Repository) updateCartTotal(db *gorm.DB, cartID uint) error {
	var total float64
	if err := db.Model(&cartpkg.CartItem{}).
		Where("cart_id = ?", cartID).
		Select("COALESCE(SUM(price * quantity), 0)").
		Scan(&total).Error; err != nil {
		return fmt.Errorf("failed to calculate total: %w", err)
	}

	return db.Model(&cartpkg.Cart{}).
		Where("id = ?", cartID).
		Update("total", total).Error
}

func (r *Repository) GetCartItem(cartID uint, itemID uint) (*cartpkg.CartItem, error) {
	var item cartpkg.CartItem
	err := r.db.Where("cart_id = ? AND id = ?", cartID, itemID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetExistingCart(sessionID string) (*cartpkg.Cart, error) {
	var c cartpkg.Cart
	result := r.db.Preload("CartItems").
		Where("session_id = ?", sessionID).
		First(&c)
	if result.Error != nil {
		return nil, result.Error
	}
	return &c, nil
}

func (r *Repository) GetAllCarts() ([]*cartpkg.Cart, error) {
	var carts []*cartpkg.Cart
	result := r.db.Preload("CartItems").Find(&carts)
	if result.Error != nil {
		return nil, result.Error
	}
	return carts, nil
}
