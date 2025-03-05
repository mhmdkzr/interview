package calculator

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	db2 "interview/pkg/db"
	"interview/pkg/entity"
	"log"
	"strconv"
)

var itemPriceMapping = map[string]float64{
	"shoe":  100,
	"purse": 200,
	"bag":   300,
	"watch": 300,
}

type CartItemForm struct {
	Product  string `form:"product"   binding:"required"`
	Quantity string `form:"quantity"  binding:"required"`
}

func AddItemToCart(c *gin.Context) {
	cookie, _ := c.Request.Cookie("ice_session_id")

	db := db2.GetDatabase()

	var isCartNew bool
	var cartEntity entity.CartEntity
	result := db.Where(fmt.Sprintf("status = '%s' AND session_id = '%s'", entity.CartOpen, cookie.Value)).First(&cartEntity)

	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Redirect(302, "/")
			return
		}
		isCartNew = true
		cartEntity = entity.CartEntity{
			SessionID: cookie.Value,
			Status:    entity.CartOpen,
		}
		db.Create(&cartEntity)
	}

	addItemForm, err := getCartItemForm(c)
	if err != nil {
		c.Redirect(302, "/?error="+err.Error())
		return
	}

	item, ok := itemPriceMapping[addItemForm.Product]
	if !ok {
		c.Redirect(302, "/?error=invalid item name")
		return
	}

	quantity, err := strconv.ParseInt(addItemForm.Quantity, 10, 0)
	if err != nil {
		c.Redirect(302, "/?error=invalid quantity")
		return
	}

	var cartItemEntity entity.CartItem
	if isCartNew {
		cartItemEntity = entity.CartItem{
			CartID:      cartEntity.ID,
			ProductName: addItemForm.Product,
			Quantity:    int(quantity),
			Price:       item * float64(quantity),
		}
		db.Create(&cartItemEntity)
	} else {
		result = db.Where(" cart_id = ? and product_name  = ?", cartEntity.ID, addItemForm.Product).First(&cartItemEntity)

		if result.Error != nil {
			if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.Redirect(302, "/")
				return
			}
			cartItemEntity = entity.CartItem{
				CartID:      cartEntity.ID,
				ProductName: addItemForm.Product,
				Quantity:    int(quantity),
				Price:       item * float64(quantity),
			}
			db.Create(&cartItemEntity)

		} else {
			cartItemEntity.Quantity += int(quantity)
			cartItemEntity.Price += item * float64(quantity)
			db.Save(&cartItemEntity)
		}
	}

	c.Redirect(302, "/")
}

func getCartItemForm(c *gin.Context) (*CartItemForm, error) {
	if c.Request.Body == nil {
		return nil, fmt.Errorf("body cannot be nil")
	}

	form := &CartItemForm{}

	if err := binding.FormPost.Bind(c.Request, form); err != nil {
		log.Println(err.Error())
		return nil, err
	}

	return form, nil
}
