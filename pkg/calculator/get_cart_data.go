package calculator

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"html/template"
	db2 "interview/pkg/db"
	"interview/pkg/entity"
	"log"
	"strings"
)

func GetCartData(c *gin.Context) {
	data := map[string]interface{}{
		"Error": c.Query("error"),
		//"cartItems": cartItems,
	}

	cookie, err := c.Request.Cookie("ice_session_id")
	if err == nil {
		data["CartItems"] = getCartItemData(cookie.Value)
	}

	html, err := renderTemplate(data)
	if err != nil {
		log.Println(err)
		c.AbortWithStatus(500)
		return
	}

	c.Header("Content-Type", "text/html")
	c.String(200, html)
}
func getCartItemData(sessionID string) (items []map[string]interface{}) {
	db := db2.GetDatabase()
	var cartEntity entity.CartEntity
	result := db.Where(fmt.Sprintf("status = '%s' AND session_id = '%s'", entity.CartOpen, sessionID)).First(&cartEntity)

	if result.Error != nil {
		return
	}

	var cartItems []entity.CartItem
	result = db.Where(fmt.Sprintf("cart_id = %d", cartEntity.ID)).Find(&cartItems)
	if result.Error != nil {
		return
	}

	for _, cartItem := range cartItems {
		item := map[string]interface{}{
			"ID":       cartItem.ID,
			"Quantity": cartItem.Quantity,
			"Price":    cartItem.Price,
			"Product":  cartItem.ProductName,
		}

		items = append(items, item)
	}
	return items
}

func renderTemplate(pageData interface{}) (string, error) {
	// Read and parse the HTML template file
	tmpl, err := template.ParseFiles("../../static/add_item_form.html")
	if err != nil {
		return "", fmt.Errorf("Error parsing template: %v ", err)
	}

	// Create a strings.Builder to store the rendered template
	var renderedTemplate strings.Builder

	err = tmpl.Execute(&renderedTemplate, pageData)
	if err != nil {
		return "", fmt.Errorf("Error parsing template: %v ", err)
	}

	// Convert the rendered template to a string
	resultString := renderedTemplate.String()

	return resultString, nil
}
