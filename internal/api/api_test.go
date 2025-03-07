package api_test

import (
	"embed"
	"fmt"
	"interview/internal/api"
	"interview/internal/cart"
	"interview/internal/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed testdata/templates
var templateFS embed.FS

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&cart.Cart{}, &cart.CartItem{})
	require.NoError(t, err)

	return db
}

// setupTestRouter creates a test Gin router with necessary middleware
func setupTestRouter(_ *testing.T, handler *api.CartHandler, db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup GORM-based sessions for testing
	store := gormsessions.NewStore(db, true, []byte("test_secret"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	// Initialize the session middleware
	router.Use(sessions.Sessions("test_session", store))

	// Add routes
	router.GET("/", handler.ShowCart)
	router.POST("/add-item", handler.AddItem)
	router.POST("/remove-item", handler.RemoveItem)

	return router
}

// setupTestHandler creates a test CartHandler with embedded templates
func setupTestHandler(_ *testing.T, db *gorm.DB) *api.CartHandler {
	testConfig := config.Config{
		SessionSecret: "test_secret",
		SessionName:   "test_session",
	}

	handler := api.NewCartHandler(db, templateFS, testConfig, "testdata/templates/*.html")

	return handler
}

// testSetup contains common test setup data
type testSetup struct {
	db      *gorm.DB
	handler *api.CartHandler
	router  *gin.Engine
}

// setupTest creates a new test environment
func setupTest(t *testing.T) *testSetup {
	t.Helper()
	db := setupTestDB(t)
	handler := setupTestHandler(t, db)
	router := setupTestRouter(t, handler, db)
	return &testSetup{
		db:      db,
		handler: handler,
		router:  router,
	}
}

// clearDatabase cleans up the test database
func (ts *testSetup) clearDatabase(t *testing.T) {
	t.Helper()
	tables := []string{"cart_items", "carts", "sessions"}
	for _, table := range tables {
		err := ts.db.Exec("DELETE FROM " + table).Error
		require.NoError(t, err)
	}
}

// createSession creates a new session and returns the session cookie
func (ts *testSetup) createSession(t *testing.T) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ts.router.ServeHTTP(w, req)

	var sessionCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "test_session" {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie, "Session cookie not found")
	return sessionCookie
}

// makeRequest performs an HTTP request with session cookie
func (ts *testSetup) makeRequest(t *testing.T, method, path string, formData url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(formData.Encode()))
	if formData != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	ts.router.ServeHTTP(w, req)
	return w
}

func TestShowCart(t *testing.T) {
	ts := setupTest(t)

	tests := []struct {
		name           string
		setupData      func(*testing.T, *api.CartHandler)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "New Session - Empty Cart",
			setupData:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   "<html lang=\"en\">",
		},
		{
			name: "Existing Session With Items",
			setupData: func(t *testing.T, h *api.CartHandler) {
				cart, err := h.GetRepo().GetOrCreateCart("test-session-id")
				require.NoError(t, err)
				err = h.GetRepo().AddCartItem(cart.ID, "shoe", 1, 10.0)
				require.NoError(t, err)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Shoe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.clearDatabase(t)

			if tt.setupData != nil {
				tt.setupData(t, ts.handler)
			}

			w := ts.makeRequest(t, http.MethodGet, "/", nil, nil)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}

func TestAddItem(t *testing.T) {
	ts := setupTest(t)

	tests := []struct {
		name           string
		formData       url.Values
		expectedStatus int
		checkResult    func(*testing.T, *api.CartHandler)
	}{
		{
			name: "Add Valid Item",
			formData: url.Values{
				"product":  []string{"shoe"},
				"quantity": []string{"2"},
			},
			expectedStatus: http.StatusFound,
			checkResult: func(t *testing.T, h *api.CartHandler) {
				carts, err := h.GetRepo().GetAllCarts()
				require.NoError(t, err)
				require.NotEmpty(t, carts)

				var cartWithItem *cart.Cart
				for _, c := range carts {
					if len(c.CartItems) > 0 {
						cartWithItem = c
						break
					}
				}

				require.NotNil(t, cartWithItem, "No cart found with items")
				require.Len(t, cartWithItem.CartItems, 1)
				assert.Equal(t, "shoe", cartWithItem.CartItems[0].ProductName)
				assert.Equal(t, 2, cartWithItem.CartItems[0].Quantity)
			},
		},
		{
			name: "Invalid Product",
			formData: url.Values{
				"product":  []string{"invalid_product"},
				"quantity": []string{"1"},
			},
			expectedStatus: http.StatusFound,
			checkResult: func(t *testing.T, h *api.CartHandler) {
				assertNoItemsInCarts(t, h)
			},
		},
		{
			name: "Invalid Quantity",
			formData: url.Values{
				"product":  []string{"shoe"},
				"quantity": []string{"-1"},
			},
			expectedStatus: http.StatusFound,
			checkResult: func(t *testing.T, h *api.CartHandler) {
				assertNoItemsInCarts(t, h)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.clearDatabase(t)

			cookie := ts.createSession(t)
			w := ts.makeRequest(t, http.MethodPost, "/add-item", tt.formData, cookie)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResult != nil {
				tt.checkResult(t, ts.handler)
			}
		})
	}
}

func TestRemoveItem(t *testing.T) {
	ts := setupTest(t)

	tests := []struct {
		name           string
		setupData      func(*testing.T, *api.CartHandler, *http.Cookie) uint
		formData       url.Values
		expectedStatus int
		checkResult    func(*testing.T, *api.CartHandler)
	}{
		{
			name: "Remove Existing Item",
			setupData: func(t *testing.T, h *api.CartHandler, cookie *http.Cookie) uint {
				// Create cart with session
				ts.makeRequest(t, http.MethodGet, "/", nil, cookie)

				// Get the cart and add an item
				carts, err := h.GetRepo().GetAllCarts()
				require.NoError(t, err)
				require.NotEmpty(t, carts)
				cart := carts[0]

				err = h.GetRepo().AddCartItem(cart.ID, "shoe", 1, 10.0)
				require.NoError(t, err)

				// Refresh cart to get the item ID
				cart, err = h.GetRepo().GetExistingCart(cart.SessionID)
				require.NoError(t, err)
				require.Len(t, cart.CartItems, 1)
				return cart.CartItems[0].ID
			},
			formData: url.Values{
				"cart_item_id": []string{"1"},
			},
			expectedStatus: http.StatusFound,
			checkResult: func(t *testing.T, h *api.CartHandler) {
				assertNoItemsInCarts(t, h)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.clearDatabase(t)

			cookie := ts.createSession(t)

			var itemID uint
			if tt.setupData != nil {
				itemID = tt.setupData(t, ts.handler, cookie)
				tt.formData.Set("cart_item_id", fmt.Sprint(itemID))
			}

			w := ts.makeRequest(t, http.MethodPost, "/remove-item", tt.formData, cookie)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResult != nil {
				tt.checkResult(t, ts.handler)
			}
		})
	}
}

// assertNoItemsInCarts verifies that no carts have any items
func assertNoItemsInCarts(t *testing.T, h *api.CartHandler) {
	t.Helper()
	carts, err := h.GetRepo().GetAllCarts()
	require.NoError(t, err)
	for _, cart := range carts {
		assert.Len(t, cart.CartItems, 0, "Cart should not have any items")
	}
}
