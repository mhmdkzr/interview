package db

import "interview/pkg/entity"

func MigrateDatabase() {
	db := GetDatabase()

	// AutoMigrate will create or update the tables based on the models
	err := db.AutoMigrate(&entity.CartEntity{}, &entity.CartItem{})
	if err != nil {
		panic(err)
	}
}
