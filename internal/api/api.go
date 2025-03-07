// Package api provides HTTP handlers, middleware, and routing for the shopping cart application.
package api

import (
	"crypto/rand"
	"embed"
	"fmt"
	"html/template"
	"interview/internal/cart"
	"interview/internal/config"
	"interview/internal/repo"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	gormSessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"gorm.io/gorm"
)

type (
	// CartHandler is used with HTTP handlers so we can inject the repository, template, and product prices.
	CartHandler struct {
		repo          *repo.Repository
		Template      *template.Template
		productPrices map[string]float64
		config        config.Config
	}

	// TemplateData contains data to be rendered in HTML templates.
	TemplateData struct {
		Error         string
		CartItems     []CartItemView
		CSRFToken     string
		CSRFFieldName template.HTML
	}

	// CartItemView represents a cart item for the view layer.
	CartItemView struct {
		ID       uint
		Product  string
		Quantity int
	}
)

// InitAPI initializes and starts the HTTP server.
func InitAPI(db *gorm.DB, templateFS embed.FS, config config.Config) {
	handler := NewCartHandler(db, templateFS, config)
	router := gin.Default()

	// Add session middleware with proper duration enforcement
	store := gormSessions.NewStore(db, true, []byte(config.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   3600, // 1 hour session duration
		HttpOnly: true,
		Secure:   false, // Set to true in production
		SameSite: http.SameSiteLaxMode,
	})

	router.Use(sessions.Sessions(config.SessionName, store))

	// Add session cleanup middleware
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		// Check session age
		created := session.Get("created")
		if created == nil {
			session.Set("created", time.Now().Unix())
			if err := session.Save(); err != nil {
				log.Printf("Error saving session: %v", err)
			}
		} else if time.Now().Unix()-created.(int64) > 3600 {
			// Session expired
			session.Clear()
			if err := session.Save(); err != nil {
				log.Printf("Error saving session after clearing: %v", err)
			}
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Next()
	})

	// CSRF Protection
	csrfMiddleware := csrf.Protect(
		[]byte(config.SessionSecret),
		csrf.Secure(false), // Set to true in production
		csrf.Path("/"),
		csrf.MaxAge(3600), // 1 hour CSRF token duration
	)

	// Add routes
	router.GET("/", handler.ShowCart)
	router.POST("/add-item", handler.AddItem)
	router.POST("/remove-item", handler.RemoveItem)

	// Add CSRF token to response headers
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-CSRF-Token", csrf.Token(c.Request))
		c.Next()
	})

	address := fmt.Sprintf(":%s", config.APIPort)
	if err := http.ListenAndServe(address, csrfMiddleware(router)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// NewCartHandler creates a new CartHandler with the given dependencies.
func NewCartHandler(db *gorm.DB, templateFS embed.FS, config config.Config) *CartHandler {
	tpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))

	// Default prices for development and testing
	defaultPrices := map[string]float64{
		"shoe":  10.0,
		"purse": 20.0,
		"bag":   30.0,
		"watch": 40.0,
	}

	return &CartHandler{
		repo:          repo.NewRepository(db),
		Template:      tpl,
		config:        config,
		productPrices: defaultPrices,
	}
}

// ShowCart displays the shopping cart page.
func (h *CartHandler) ShowCart(c *gin.Context) {
	session := sessions.Default(c)
	data := TemplateData{}

	flashes := session.Flashes()
	if len(flashes) > 0 {
		data.Error = flashes[0].(string)
		session.Save()
	}

	// Get or create a unique session ID
	sessionID := session.Get("session_id")
	if sessionID == nil {
		// Generate a new unique session ID
		sessionID = generateSessionID()
		session.Set("session_id", sessionID)
		if err := session.Save(); err != nil {
			log.Printf("Failed to save session: %v", err)
			data.Error = "Failed to create session"
			h.RenderTemplate(c, data)
			return
		}
	}

	cart, err := h.repo.GetOrCreateCart(sessionID.(string))
	if err != nil {
		data.Error = "Failed to load cart"
	} else {
		data.CartItems = h.CreateCartItemViews(cart.CartItems)
	}

	h.RenderTemplate(c, data)
}

func generateSessionID() string {
	// Generate a random session ID using crypto/rand
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// AddItem adds a product to the user's cart.
func (h *CartHandler) AddItem(c *gin.Context) {
	session := sessions.Default(c)

	product := sanitizeProductName(c.PostForm("product"))
	quantityStr := c.PostForm("quantity")

	if product == "" {
		session.AddFlash("Please select a product")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Validate product name against allowed list
	if !isValidProduct(product) {
		session.AddFlash("Invalid product selected")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	if quantityStr == "" {
		session.AddFlash("Please enter a quantity")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity < 1 {
		session.AddFlash("Quantity must be a valid number greater than 0")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	price, err := h.GetProductPrice(product)
	if err != nil {
		session.AddFlash(err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	sessionID := session.Get("session_id")
	if sessionID == nil {
		session.AddFlash("Invalid session")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	userCart, err := h.repo.GetOrCreateCart(sessionID.(string))
	if err != nil {
		session.AddFlash("Failed to load cart")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Validate cart ownership
	if userCart.SessionID != sessionID.(string) {
		session.AddFlash("Unauthorized cart access")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	if err := h.repo.AddCartItem(userCart.ID, product, quantity, price); err != nil {
		session.AddFlash("Failed to add item to cart")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// RemoveItem removes an item from the user's cart.
func (h *CartHandler) RemoveItem(c *gin.Context) {
	session := sessions.Default(c)

	itemID, err := strconv.ParseUint(c.PostForm("cart_item_id"), 10, 32)
	if err != nil {
		session.AddFlash("Invalid item ID")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	sessionID := session.Get("session_id")
	if sessionID == nil {
		session.AddFlash("Invalid session")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	userCart, err := h.repo.GetExistingCart(sessionID.(string))
	if err != nil {
		session.AddFlash("Cart not found")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Validate cart ownership
	if userCart.SessionID != sessionID.(string) {
		session.AddFlash("Unauthorized cart access")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Validate item belongs to cart
	item, err := h.repo.GetCartItem(userCart.ID, uint(itemID))
	if err != nil || item == nil {
		session.AddFlash("Item not found")
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	if err := h.repo.RemoveCartItem(userCart.ID, uint(itemID)); err != nil {
		session.AddFlash(err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// GetProductPrice returns the price of a product by name.
func (h *CartHandler) GetProductPrice(name string) (float64, error) {
	if h.productPrices != nil {
		price, ok := h.productPrices[name]
		if !ok {
			return 0, fmt.Errorf("product not found: %s", name)
		}
		return price, nil
	}

	price, ok := h.productPrices[name]
	if !ok {
		return 0, fmt.Errorf("product not found: %s", name)
	}

	return price, nil
}

// CreateCartItemViews converts cart items to view models
func (h *CartHandler) CreateCartItemViews(items []cart.CartItem) []CartItemView {
	views := make([]CartItemView, len(items))
	for i, item := range items {
		views[i] = CartItemView{
			ID:       item.ID,
			Product:  item.ProductName,
			Quantity: item.Quantity,
		}
	}
	return views
}

// RenderTemplate renders the cart template with the given data
func (h *CartHandler) RenderTemplate(c *gin.Context, data TemplateData) {
	data.CSRFToken = csrf.Token(c.Request)
	data.CSRFFieldName = csrf.TemplateField(c.Request)
	if err := h.Template.ExecuteTemplate(c.Writer, "cart.html", data); err != nil {
		log.Printf("Failed to render template: %v", err)
		http.Error(c.Writer, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SetProductPrices sets the product prices map for testing.
func (h *CartHandler) SetProductPrices(prices map[string]float64) {
	h.productPrices = prices
}

// Helper functions for input validation and sanitization
func sanitizeProductName(name string) string {
	// TODO: use external library for sanitization
	// Convert to lowercase and remove any characters that aren't alphanumeric or underscore
	sanitized := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			sanitized += string(char)
		} else if char >= 'A' && char <= 'Z' {
			sanitized += string(char + 32) // Convert to lowercase
		}
	}
	return sanitized
}

func isValidProduct(name string) bool {
	return name == "shoe" || name == "purse" || name == "bag" || name == "watch"
}
