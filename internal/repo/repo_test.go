package repo_test

import (
	"errors"
	cartpkg "interview/internal/cart"
	"interview/internal/repo"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&cartpkg.Cart{}, &cartpkg.CartItem{})
	require.NoError(t, err)

	return db
}

func TestGetOrCreateCart(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("creates new cart when none exists", func(t *testing.T) {
		sessionID := "test-session-1"
		cart, err := repo.GetOrCreateCart(sessionID)

		require.NoError(t, err)
		assert.Equal(t, sessionID, cart.SessionID)
		assert.Equal(t, cartpkg.StatusOpen, cart.Status)
		assert.Empty(t, cart.CartItems)
	})

	t.Run("returns existing cart", func(t *testing.T) {
		sessionID := "test-session-2"
		cart1, err := repo.GetOrCreateCart(sessionID)
		require.NoError(t, err)

		cart2, err := repo.GetOrCreateCart(sessionID)
		require.NoError(t, err)

		assert.Equal(t, cart1.ID, cart2.ID)
		assert.Equal(t, sessionID, cart2.SessionID)
	})
}

func TestAddCartItem(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("adds new item to cart", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session")
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 1, 10.0)
		require.NoError(t, err)

		updatedCart, err := repo.GetExistingCart("test-session")
		require.NoError(t, err)
		require.Len(t, updatedCart.CartItems, 1)
		assert.Equal(t, "test-product", updatedCart.CartItems[0].ProductName)
		assert.Equal(t, 1, updatedCart.CartItems[0].Quantity)
		assert.Equal(t, 10.0, updatedCart.CartItems[0].Price)
		assert.Equal(t, 10.0, updatedCart.Total)
	})

	t.Run("updates quantity for existing item", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session-2")
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 1, 10.0)
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 2, 10.0)
		require.NoError(t, err)

		updatedCart, err := repo.GetExistingCart("test-session-2")
		require.NoError(t, err)
		require.Len(t, updatedCart.CartItems, 1)
		assert.Equal(t, 3, updatedCart.CartItems[0].Quantity)
		assert.Equal(t, 30.0, updatedCart.Total)
	})

	t.Run("fails for non-existent cart", func(t *testing.T) {
		err := repo.AddCartItem(9999, "test-product", 1, 10.0)
		assert.Error(t, err)
	})
}

func TestRemoveCartItem(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("removes item from cart", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session")
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 1, 10.0)
		require.NoError(t, err)

		updatedCart, err := repo.GetExistingCart("test-session")
		require.NoError(t, err)
		require.Len(t, updatedCart.CartItems, 1)

		err = repo.RemoveCartItem(cart.ID, updatedCart.CartItems[0].ID)
		require.NoError(t, err)

		finalCart, err := repo.GetExistingCart("test-session")
		require.NoError(t, err)
		assert.Empty(t, finalCart.CartItems)
		assert.Equal(t, 0.0, finalCart.Total)
	})

	t.Run("fails for non-existent item", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session-2")
		require.NoError(t, err)

		err = repo.RemoveCartItem(cart.ID, 9999)
		assert.Error(t, err)
	})
}

func TestGetCartItem(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("gets existing item", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session")
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 1, 10.0)
		require.NoError(t, err)

		updatedCart, err := repo.GetExistingCart("test-session")
		require.NoError(t, err)

		item, err := repo.GetCartItem(cart.ID, updatedCart.CartItems[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "test-product", item.ProductName)
		assert.Equal(t, 1, item.Quantity)
		assert.Equal(t, 10.0, item.Price)
	})

	t.Run("returns error for non-existent item", func(t *testing.T) {
		cart, err := repo.GetOrCreateCart("test-session-2")
		require.NoError(t, err)

		_, err = repo.GetCartItem(cart.ID, 9999)
		assert.Error(t, err)
	})
}

func TestGetAllCarts(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("returns empty list when no carts exist", func(t *testing.T) {
		carts, err := repo.GetAllCarts()
		require.NoError(t, err)
		assert.Empty(t, carts)
	})

	t.Run("returns all existing carts", func(t *testing.T) {
		cart1, err := repo.GetOrCreateCart("test-session-1")
		require.NoError(t, err)
		err = repo.AddCartItem(cart1.ID, "product-1", 1, 10.0)
		require.NoError(t, err)

		cart2, err := repo.GetOrCreateCart("test-session-2")
		require.NoError(t, err)
		err = repo.AddCartItem(cart2.ID, "product-2", 2, 20.0)
		require.NoError(t, err)

		carts, err := repo.GetAllCarts()
		require.NoError(t, err)
		assert.Len(t, carts, 2)
	})
}

func TestGetExistingCart(t *testing.T) {
	db := setupTestDB(t)
	repo := repo.NewRepository(db)

	t.Run("returns error for non-existent cart", func(t *testing.T) {
		_, err := repo.GetExistingCart("non-existent-session")
		assert.Error(t, err)
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	})

	t.Run("returns existing cart", func(t *testing.T) {
		sessionID := "test-session"
		cart, err := repo.GetOrCreateCart(sessionID)
		require.NoError(t, err)

		err = repo.AddCartItem(cart.ID, "test-product", 1, 10.0)
		require.NoError(t, err)

		existingCart, err := repo.GetExistingCart(sessionID)
		require.NoError(t, err)
		assert.Equal(t, cart.ID, existingCart.ID)
		assert.Equal(t, sessionID, existingCart.SessionID)
		require.Len(t, existingCart.CartItems, 1)
		assert.Equal(t, "test-product", existingCart.CartItems[0].ProductName)
	})
}
