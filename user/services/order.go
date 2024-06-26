package services

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"shop/auth"
	"shop/database"
	"shop/entities"
	"shop/helpers"
	"shop/helpers/zarinpal"
	"sort"

	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ordersCollection *mongo.Collection = database.GetCollection(database.DB, "orders")
var XCollection *mongo.Collection = database.GetCollection(database.DB, "users")
var produCollection *mongo.Collection = database.GetCollection(database.DB, "products")
var countersCollection *mongo.Collection = database.GetCollection(database.DB, "identitycounters")
var addusersCollection *mongo.Collection = database.GetCollection(database.DB, "brandschemas")

//@Summary GET Order
//@Description Get Order by OrderId
//@Tags Orders
//@Accept json
//@Produce json
//@Param Authorization header string true "authorization" format("Bearer your_actual_token_here")
//@Success 200 "Success"
//@Router /api/users/orders [get]
func Findorders(c *gin.Context) {
	// if err := auth.CheckUserType(c, "admin"); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 	return

	// }
	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	claims, ok := tokenClaims.(*auth.SignedUserDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	ID := claims.Id

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var orders []entities.Order
	defer cancel()

	results, err := ordersCollection.Find(ctx, bson.M{"userId": ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"massage": "Not Find Collection"})
		return
	}
	//results.Close(ctx)
	for results.Next(ctx) {
		var order entities.Order
		err := results.Decode(&order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return

		}
		orders = append(orders, order)

	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "orders", "body": orders})

}

//@Summary Post Order
//@Description Post a Product to order
//@Tags Orders
//@Accept json
//@Produce json
//@Param Authorization header string true "authorization" format("Bearer your_actual_token_here")
//@Param order body entities.Order true "Order object to be Ordered"
//@Success 200 "Success"
//@Router /api/users/orders [post]
func AddOrder(c *gin.Context) {
	var order entities.Order

	// Retrieving token claims from context
	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	claims, ok := tokenClaims.(*auth.SignedUserDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	username := claims.Username
	id := claims.Id

	// Fetching user's address from the database
	var users entities.Users
	err := usersCollection.FindOne(c, bson.M{"username": username}).Decode(&users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"errorAddress": err.Error()})
		return
	}

	// Assigning user's address to order
	for _, user := range users.Address {
		order.Address = entities.Addrs{
			Id:         user.Id,
			Address:    user.Address,
			City:       user.City,
			PostalCode: user.PostalCode,
			State:      user.State,
		}
	}

	// Incrementing order ID from counter collection
	counter := struct {
		Count int `bson:"count"`
	}{}
	oid, err := primitive.ObjectIDFromHex("603e7dcc0e4e3d00128812cc")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"errorCounter": err.Error()})
		return
	}

	err = countersCollection.FindOneAndUpdate(
		c,
		bson.M{"_id": oid},
		bson.M{"$inc": bson.M{"count": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&counter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"errorCounter": err.Error()})
		return
	}

	order.Id = counter.Count
	order.StartDate = time.Now()
	order.Status = "none"
	order.PaymentStatus = "unpaid"
	order.PaymentId = ""
	order.UserId = id
	order.IsCoupon = false
	order.Message = ""
	order.TotalDiscount = 0
	order.PostalCost = 40000
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.V = 0

	// Fetching user's cart from the database
	var cart entities.Catrs
	err = cartCollection.FindOne(c, bson.M{"username": username}).Decode(&cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "not found this user in cart"})
		return
	}

	order.Mix = cart.Mix

	// Calculating total price for mix products
	// var totalPrice int // Assuming totalPrice is an integer

	// If cart has mix, calculate total price from mix products
	if len(cart.Mix) == 0 {

		var totalPrice int
		var totalDiscount float64

		// Iterate through cart products
		for _, product := range cart.Products {
			productID := product.ProductId
			variationKey := product.VariationsKey
			productQuantity := product.Quantity

			var retrievedProduct entities.Products
			err := proCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"errorProduct": err.Error()})
				return
			}

			if len(variationKey) == 0 {
				// If no variationsKey, proceed with the order creation for products without variations
				orderProduct := entities.Product{
					Quantity:        productQuantity,
					Id:              retrievedProduct.ID,
					Name:            retrievedProduct.Name,
					Price:           retrievedProduct.Price,
					ProductId:       product.ProductId,
					DiscountPercent: float64(retrievedProduct.DiscountPercent),
				}

				// Calculate discount and total price for the product without variations
				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
				p := orderProduct.Price*productQuantity - (discount * productQuantity)

				// Update order details for products without variations
				order.Products = append(order.Products, orderProduct)
				order.TotalQuantity += productQuantity
				totalPrice += p
				totalDiscount += float64(discount * productQuantity)
			} else {
				// If variationsKey is not empty, proceed with variation check for products with variations
				if retrievedProduct.ID.IsZero() {
					continue
				}

				var selectedVariation entities.Variation
				for _, variation := range retrievedProduct.Variations {
					sort.Ints(variationKey) // Sort variationKey
					sort.Ints(variation.Keys)
					if reflect.DeepEqual(variation.Keys, variationKey) {
						selectedVariation = variation
						break
					}
				}

				if selectedVariation.Id.IsZero() {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Selected variation not found"})
					return
				}

				// Create order product for products with variations
				orderProduct := entities.Product{
					Quantity:        productQuantity,
					Id:              retrievedProduct.ID,
					Name:            retrievedProduct.Name,
					Price:           selectedVariation.Price,
					VariationKey:    selectedVariation.Keys,
					ProductId:       product.ProductId,
					DiscountPercent: float64(retrievedProduct.DiscountPercent),
				}

				// Calculate discount and total price for the product with variations
				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
				p := orderProduct.Price*productQuantity - (discount * productQuantity)

				// Update order details for products with variations
				order.Products = append(order.Products, orderProduct)
				order.TotalQuantity += productQuantity
				totalPrice += p
				totalDiscount += float64(discount * productQuantity)
			}
		}

		// Add postal cost once
		totalPrice += order.PostalCost

		// Update order details with total price and total discount
		order.TotalPrice = totalPrice
		order.TotalDiscount = totalDiscount
	} else if len(cart.Products) == 0 {
		// If cart has mix but no individual products, calculate total price from mix products
		var mixes []entities.Mixes
		for _, mixID := range order.Mix {
			var mix entities.Mixes

			// Find the mix document by its ID
			err := mixesCollection.FindOne(c, bson.M{"_id": mixID}).Decode(&mix)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find mix", "details": err.Error()})
				return
			}
			mixes = append(mixes, mix)
		}

		var totalPrice int
		for _, mix := range mixes {
			cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
				return
			}

			defer cur.Close(c)

			index := 0
			// Iterate through mix products to calculate subtotal for each
			for cur.Next(c) {
				var mixProduct entities.MixProducts
				if err := cur.Decode(&mixProduct); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// Ensure that the index does not exceed the length of the balance array
				if index >= len(mix.Balance) {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "index out of range"})
					return
				}
				subtotal := 0
				mixBalance := mix.Balance[index] * mix.Weight
				mixDevide := float64(mixBalance) / 10000
				money := mixDevide * float64(mixProduct.Price)
				subtotal += int(money)
				// Calculate subtotal for the mix product using the correct index
				// subtotal := (mix.Balance[index] * mix.Weight) / 10000 * mixProduct.Price
				totalPrice += subtotal

				// Increment the index for the next mix product
				index++
			}

			if err := cur.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		if order.Products == nil {
			order.Products = make([]entities.Product, 0)
		}

		// Assign total price to order
		order.TotalPrice = totalPrice + order.PostalCost
	} else if len(cart.Products) > 0 && len(cart.Mix) > 0 {

		var tm int
		var totalPrice1 int
		var totalDiscount float64

		// Iterate through cart products
		for _, product := range cart.Products {
			productID := product.ProductId
			variationKey := product.VariationsKey
			productQuantity := product.Quantity

			var retrievedProduct entities.Products
			err := proCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"errorProduct": err.Error()})
				return
			}

			if len(variationKey) == 0 {
				// If no variationsKey, proceed with the order creation for products without variations
				orderProduct := entities.Product{
					Quantity:        productQuantity,
					Id:              retrievedProduct.ID,
					Name:            retrievedProduct.Name,
					Price:           retrievedProduct.Price,
					ProductId:       product.ProductId,
					DiscountPercent: float64(retrievedProduct.DiscountPercent),
				}

				// Calculate discount and total price for the product without variations
				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
				p := orderProduct.Price*productQuantity - (discount * productQuantity)

				// Update order details for products without variations
				order.Products = append(order.Products, orderProduct)
				order.TotalQuantity += productQuantity
				totalPrice1 += p
				totalDiscount += float64(discount * productQuantity)
			} else {
				// If variationsKey is not empty, proceed with variation check for products with variations
				if retrievedProduct.ID.IsZero() {
					continue
				}

				var selectedVariation entities.Variation
				for _, variation := range retrievedProduct.Variations {
					sort.Ints(variationKey) // Sort variationKey
					sort.Ints(variation.Keys)
					if reflect.DeepEqual(variation.Keys, variationKey) {
						selectedVariation = variation
						break
					}
				}

				if selectedVariation.Id.IsZero() {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Selected variation not found"})
					return
				}

				// Create order product for products with variations
				orderProduct := entities.Product{
					Quantity:        productQuantity,
					Id:              retrievedProduct.ID,
					Name:            retrievedProduct.Name,
					Price:           selectedVariation.Price,
					VariationKey:    selectedVariation.Keys,
					ProductId:       product.ProductId,
					DiscountPercent: float64(retrievedProduct.DiscountPercent),
				}

				// Calculate discount and total price for the product with variations
				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
				p := orderProduct.Price*productQuantity - (discount * productQuantity)

				// Update order details for products with variations
				order.Products = append(order.Products, orderProduct)
				order.TotalQuantity += productQuantity
				totalPrice1 += p
				totalDiscount += float64(discount * productQuantity)
			}
		}

		// Add postal cost once
		// totalPrice1 += order.PostalCost

		// Update order details with total price and total discount
		order.TotalPrice = totalPrice1
		order.TotalDiscount = totalDiscount
		// If cart has mix but no individual products, calculate total price from mix products
		var mixes []entities.Mixes
		for _, mixID := range order.Mix {
			var mix entities.Mixes

			// Find the mix document by its ID
			err := mixesCollection.FindOne(c, bson.M{"_id": mixID}).Decode(&mix)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find mix", "details": err.Error()})
				return
			}
			mixes = append(mixes, mix)
		}

		var totalPrice2 int
		for _, mix := range mixes {
			cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
				return
			}

			defer cur.Close(c)

			index := 0
			// Iterate through mix products to calculate subtotal for each
			for cur.Next(c) {
				var mixProduct entities.MixProducts
				if err := cur.Decode(&mixProduct); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// Ensure that the index does not exceed the length of the balance array
				if index >= len(mix.Balance) {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "index out of range"})
					return
				}
				subtotal := 0
				mixBalance := mix.Balance[index] * mix.Weight
				mixDevide := float64(mixBalance) / 10000
				money := mixDevide * float64(mixProduct.Price)
				subtotal += int(money)
				// Calculate subtotal for the mix product using the correct index
				// subtotal := (mix.Balance[index] * mix.Weight) / 10000 * mixProduct.Price
				totalPrice2 += subtotal

				// Increment the index for the next mix product
				index++
			}

			if err := cur.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		if order.Products == nil {
			order.Products = make([]entities.Product, 0)
		}
		tm = totalPrice2
		// Assign total price to order
		order.TotalPrice = tm + totalPrice1 + order.PostalCost

	}

	// order.TotalPrice = totalPrice
	// Initialize Products field if nil

	// Delete cart after order creation
	_, err = cartCollection.DeleteOne(c, bson.M{"username": username})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove cart"})
		return
	}

	// Insert order into the database
	_, err = ordersCollection.InsertOne(c, order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Respond with success message and order ID
	c.JSON(http.StatusOK, gin.H{"message": "order_added", "success": true, "body": gin.H{"orderId": order.Id}})
}

// func AddOrder(c *gin.Context) {
// 	var order entities.Order

// 	// Retrieving token claims from context
// 	tokenClaims, exists := c.Get("tokenClaims")
// 	if !exists {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
// 		return
// 	}

// 	claims, ok := tokenClaims.(*auth.SignedUserDetails)
// 	if !ok {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
// 		return
// 	}

// 	username := claims.Username
// 	id := claims.Id

// 	// Fetching user's address from the database
// 	var users entities.Users
// 	err := usersCollection.FindOne(c, bson.M{"username": username}).Decode(&users)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"errorAddress": err.Error()})
// 		return
// 	}

// 	// Assigning user's address to order
// 	for _, user := range users.Address {
// 		order.Address = entities.Addrs{
// 			Id:         user.Id,
// 			Address:    user.Address,
// 			City:       user.City,
// 			PostalCode: user.PostalCode,
// 			State:      user.State,
// 		}
// 	}

// 	// Incrementing order ID from counter collection
// 	counter := struct {
// 		Count int `bson:"count"`
// 	}{}
// 	oid, err := primitive.ObjectIDFromHex("603e7dcc0e4e3d00128812cc")
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"errorCounter": err.Error()})
// 		return
// 	}

// 	err = countersCollection.FindOneAndUpdate(
// 		c,
// 		bson.M{"_id": oid},
// 		bson.M{"$inc": bson.M{"count": 1}},
// 		options.FindOneAndUpdate().SetReturnDocument(options.After),
// 	).Decode(&counter)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"errorCounter": err.Error()})
// 		return
// 	}

// 	order.Id = counter.Count
// 	order.StartDate = time.Now()
// 	order.Status = "none"
// 	order.PaymentStatus = "unpaid"
// 	order.PaymentId = ""
// 	order.UserId = id
// 	order.IsCoupon = false
// 	order.Message = ""
// 	order.TotalDiscount = 0
// 	order.PostalCost = 40000
// 	order.CreatedAt = time.Now()
// 	order.UpdatedAt = time.Now()
// 	order.V = 0

// 	// Fetching user's cart from the database
// 	var cart entities.Catrs
// 	err = cartCollection.FindOne(c, bson.M{"username": username}).Decode(&cart)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "not found this user in cart"})
// 		return
// 	}

// 	order.Mix = cart.Mix

// 	// Calculating total price for mix products
// 	// var totalPrice int // Assuming totalPrice is an integer

// 	// If cart has mix, calculate total price from mix products
// 	if cart.Mix.IsZero() {

// 		var totalPrice int
// 		var totalDiscount float64

// 		// Iterate through cart products
// 		for _, product := range cart.Products {
// 			productID := product.ProductId
// 			variationKey := product.VariationsKey
// 			productQuantity := product.Quantity

// 			var retrievedProduct entities.Products
// 			err := proCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
// 			if err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"errorProduct": err.Error()})
// 				return
// 			}

// 			if len(variationKey) == 0 {
// 				// If no variationsKey, proceed with the order creation for products without variations
// 				orderProduct := entities.Product{
// 					Quantity:        productQuantity,
// 					Id:              retrievedProduct.ID,
// 					Name:            retrievedProduct.Name,
// 					Price:           retrievedProduct.Price,
// 					ProductId:       product.ProductId,
// 					DiscountPercent: float64(retrievedProduct.DiscountPercent),
// 				}

// 				// Calculate discount and total price for the product without variations
// 				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
// 				p := orderProduct.Price*productQuantity - (discount * productQuantity)

// 				// Update order details for products without variations
// 				order.Products = append(order.Products, orderProduct)
// 				order.TotalQuantity += productQuantity
// 				totalPrice += p
// 				totalDiscount += float64(discount * productQuantity)
// 			} else {
// 				// If variationsKey is not empty, proceed with variation check for products with variations
// 				if retrievedProduct.ID.IsZero() {
// 					continue
// 				}

// 				var selectedVariation entities.Variation
// 				for _, variation := range retrievedProduct.Variations {
// 					sort.Ints(variationKey) // Sort variationKey
// 					sort.Ints(variation.Keys)
// 					if reflect.DeepEqual(variation.Keys, variationKey) {
// 						selectedVariation = variation
// 						break
// 					}
// 				}

// 				if selectedVariation.Id.IsZero() {
// 					c.JSON(http.StatusInternalServerError, gin.H{"error": "Selected variation not found"})
// 					return
// 				}

// 				// Create order product for products with variations
// 				orderProduct := entities.Product{
// 					Quantity:        productQuantity,
// 					Id:              retrievedProduct.ID,
// 					Name:            retrievedProduct.Name,
// 					Price:           selectedVariation.Price,
// 					VariationKey:    selectedVariation.Keys,
// 					ProductId:       product.ProductId,
// 					DiscountPercent: float64(retrievedProduct.DiscountPercent),
// 				}

// 				// Calculate discount and total price for the product with variations
// 				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
// 				p := orderProduct.Price*productQuantity - (discount * productQuantity)

// 				// Update order details for products with variations
// 				order.Products = append(order.Products, orderProduct)
// 				order.TotalQuantity += productQuantity
// 				totalPrice += p
// 				totalDiscount += float64(discount * productQuantity)
// 			}
// 		}

// 		// Add postal cost once
// 		totalPrice += order.PostalCost

// 		// Update order details with total price and total discount
// 		order.TotalPrice = totalPrice
// 		order.TotalDiscount = totalDiscount
// 	} else if len(cart.Products) == 0 {
// 		// If cart has mix but no individual products, calculate total price from mix products
// 		var mix entities.Mixes
// 		err := mixesCollection.FindOne(c, bson.M{"_id": order.Mix}).Decode(&mix)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "mix not found"})
// 			return
// 		}

// 		// Fetch mix products details
// 		cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
// 			return
// 		}

// 		defer cur.Close(c)
// 		var totalPrice int
// 		index := 0
// 		// Iterate through mix products to calculate subtotal for each
// 		for cur.Next(c) {
// 			var mixProduct entities.MixProducts
// 			if err := cur.Decode(&mixProduct); err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 				return
// 			}

// 			// Ensure that the index does not exceed the length of the balance array
// 			if index >= len(mix.Balance) {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "index out of range"})
// 				return
// 			}
// 			subtotal := 0
// 			mixBalance := mix.Balance[index] * mix.Weight
// 			mixDevide := float64(mixBalance) / 10000
// 			money := mixDevide * float64(mixProduct.Price)
// 			subtotal += int(money)
// 			// Calculate subtotal for the mix product using the correct index
// 			// subtotal := (mix.Balance[index] * mix.Weight) / 10000 * mixProduct.Price
// 			totalPrice += subtotal

// 			// Increment the index for the next mix product
// 			index++
// 		}

// 		if err := cur.Err(); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}
// 		if order.Products == nil {
// 			order.Products = make([]entities.Product, 0)
// 		}

// 		// Assign total price to order
// 		order.TotalPrice = totalPrice + order.PostalCost
// 	} else if len(cart.Products) > 0 && !cart.Mix.IsZero() {

// 		var tm int
// 		var totalPrice1 int
// 		var totalDiscount float64

// 		// Iterate through cart products
// 		for _, product := range cart.Products {
// 			productID := product.ProductId
// 			variationKey := product.VariationsKey
// 			productQuantity := product.Quantity

// 			var retrievedProduct entities.Products
// 			err := proCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
// 			if err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"errorProduct": err.Error()})
// 				return
// 			}

// 			if len(variationKey) == 0 {
// 				// If no variationsKey, proceed with the order creation for products without variations
// 				orderProduct := entities.Product{
// 					Quantity:        productQuantity,
// 					Id:              retrievedProduct.ID,
// 					Name:            retrievedProduct.Name,
// 					Price:           retrievedProduct.Price,
// 					ProductId:       product.ProductId,
// 					DiscountPercent: float64(retrievedProduct.DiscountPercent),
// 				}

// 				// Calculate discount and total price for the product without variations
// 				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
// 				p := orderProduct.Price*productQuantity - (discount * productQuantity)

// 				// Update order details for products without variations
// 				order.Products = append(order.Products, orderProduct)
// 				order.TotalQuantity += productQuantity
// 				totalPrice1 += p
// 				totalDiscount += float64(discount * productQuantity)
// 			} else {
// 				// If variationsKey is not empty, proceed with variation check for products with variations
// 				if retrievedProduct.ID.IsZero() {
// 					continue
// 				}

// 				var selectedVariation entities.Variation
// 				for _, variation := range retrievedProduct.Variations {
// 					sort.Ints(variationKey) // Sort variationKey
// 					sort.Ints(variation.Keys)
// 					if reflect.DeepEqual(variation.Keys, variationKey) {
// 						selectedVariation = variation
// 						break
// 					}
// 				}

// 				if selectedVariation.Id.IsZero() {
// 					c.JSON(http.StatusInternalServerError, gin.H{"error": "Selected variation not found"})
// 					return
// 				}

// 				// Create order product for products with variations
// 				orderProduct := entities.Product{
// 					Quantity:        productQuantity,
// 					Id:              retrievedProduct.ID,
// 					Name:            retrievedProduct.Name,
// 					Price:           selectedVariation.Price,
// 					VariationKey:    selectedVariation.Keys,
// 					ProductId:       product.ProductId,
// 					DiscountPercent: float64(retrievedProduct.DiscountPercent),
// 				}

// 				// Calculate discount and total price for the product with variations
// 				discount := orderProduct.Price * int(orderProduct.DiscountPercent) / 100
// 				p := orderProduct.Price*productQuantity - (discount * productQuantity)

// 				// Update order details for products with variations
// 				order.Products = append(order.Products, orderProduct)
// 				order.TotalQuantity += productQuantity
// 				totalPrice1 += p
// 				totalDiscount += float64(discount * productQuantity)
// 			}
// 		}

// 		// Add postal cost once
// 		// totalPrice1 += order.PostalCost

// 		// Update order details with total price and total discount
// 		order.TotalPrice = totalPrice1
// 		order.TotalDiscount = totalDiscount
// 		// If cart has mix but no individual products, calculate total price from mix products
// 		var mix entities.Mixes
// 		err := mixesCollection.FindOne(c, bson.M{"_id": order.Mix}).Decode(&mix)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "mix not found"})
// 			return
// 		}

// 		// Fetch mix products details
// 		cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
// 			return
// 		}

// 		defer cur.Close(c)
// 		var totalPrice2 int
// 		index := 0
// 		// Iterate through mix products to calculate subtotal for each
// 		for cur.Next(c) {
// 			var mixProduct entities.MixProducts
// 			if err := cur.Decode(&mixProduct); err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 				return
// 			}

// 			// Ensure that the index does not exceed the length of the balance array
// 			if index >= len(mix.Balance) {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": "index out of range"})
// 				return
// 			}
// 			subtotal := 0
// 			mixBalance := mix.Balance[index] * mix.Weight
// 			mixDevide := float64(mixBalance) / 10000
// 			money := mixDevide * float64(mixProduct.Price)
// 			subtotal += int(money)
// 			// Calculate subtotal for the mix product using the correct index
// 			// subtotal := (mix.Balance[index] * mix.Weight) / 10000 * mixProduct.Price
// 			totalPrice2 += subtotal

// 			// Increment the index for the next mix product
// 			index++
// 		}

// 		if err := cur.Err(); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}
// 		if order.Products == nil {
// 			order.Products = make([]entities.Product, 0)
// 		}
// 		tm = totalPrice2
// 		// Assign total price to order
// 		order.TotalPrice = tm + totalPrice1 + order.PostalCost

// 	}

// 	// order.TotalPrice = totalPrice
// 	// Initialize Products field if nil

// 	// Delete cart after order creation
// 	_, err = cartCollection.DeleteOne(c, bson.M{"username": username})
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove cart"})
// 		return
// 	}

// 	// Insert order into the database
// 	_, err = ordersCollection.InsertOne(c, order)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Respond with success message and order ID
// 	c.JSON(http.StatusOK, gin.H{"message": "order_added", "success": true, "body": gin.H{"orderId": order.Id}})
// }

//***************************************************************************

//@Summary GET Order
//@Description Get Order by OrderId
//@Tags Orders
//@Accept json
//@Produce json
//@Param Authorization header string true "authorization" format("Bearer your_actual_token_here")
//@Param id path string true "Order ID to Get from an Order" format("hexadecimal ObjectId")
//@Success 200 "Success"
//@Router /api/users/orders/{id} [get]
func GetOrderByID(c *gin.Context) {
	orderID := c.Param("id")

	// Convert orderID to int
	orderIntID, err := strconv.Atoi(orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	claims, ok := tokenClaims.(*auth.SignedUserDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	userID := claims.Id

	var orders entities.Order
	err = ordersCollection.FindOne(c, bson.M{"_id": orderIntID, "userId": userID}).Decode(&orders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"messageOrder": err.Error()})
		return
	}

	// // Check if the order has a mix
	if len(orders.Products) == 0 {
		var mixes []entities.Mixes
		var mixProductsMap = make(map[string][]entities.MixProducts) // Map to store mix products by mix ID
		for _, mixID := range orders.Mix {
			var mix entities.Mixes

			// Find the mix document by its ID
			err := mixesCollection.FindOne(c, bson.M{"_id": mixID}).Decode(&mix)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find mix", "details": err.Error()})
				return
			}
			mixes = append(mixes, mix)

			// Fetch mix products for the current mix
			var mixProducts []entities.MixProducts
			cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
				return
			}

			defer cur.Close(c)

			// Iterate through the cursor to decode each mix product
			for cur.Next(c) {
				var mixProduct entities.MixProducts
				if err := cur.Decode(&mixProduct); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				mixProducts = append(mixProducts, mixProduct)
			}
			if err := cur.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Store mix products in the map with mix ID as the key (convert mix ID to string)
			mixProductsMap[mix.ID.Hex()] = mixProducts
		}

		// Construct mix array for the response body
		var mixArray []gin.H
		for _, mix := range mixes {
			mixProductArray, ok := mixProductsMap[mix.ID.Hex()] // Convert mix ID to string
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "mix products not found for mix ID: " + mix.ID.Hex()})
				return
			}
			mixDetails := gin.H{
				"_id":       mix.ID,
				"name":      mix.Name,
				"weight":    mix.Weight,
				"pattern":   mix.Pattern,
				"userId":    mix.UserId,
				"createdAt": mix.CreatedAt,
				"updatedAt": mix.UpdatedAt,
				"__v":       mix.V,
				"balance":   mix.Balance,
				"products":  mixProductArray,
			}
			mixArray = append(mixArray, mixDetails)
		}

		// Construct the response body including mixes and other order details
		responseBody := gin.H{
			"isCoupon":      orders.IsCoupon,
			"startDate":     orders.StartDate,
			"status":        orders.Status,
			"paymentStatus": orders.PaymentStatus,
			"message":       orders.Message,
			"_id":           orders.Id,
			"totalPrice":    orders.TotalPrice,
			"totalDiscount": orders.TotalDiscount,
			"totalQuantity": orders.TotalQuantity,
			"postalCost":    orders.PostalCost,
			"userId":        orders.UserId,
			"products":      orders.Products,
			"jStartDate":    orders.JStartDate,
			"address":       orders.Address,
			"mixes":         mixArray,
			"createdAt":     orders.CreatedAt,
			"updatedAt":     orders.UpdatedAt,
			"__v":           orders.V,
		}

		// Send the response
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})

	} else if len(orders.Mix) == 0 {
		// If the order doesn't have a mix, handle it as usual
		// Construct the response body without mix details
		responseBody := gin.H{
			"isCoupon":      orders.IsCoupon,
			"startDate":     orders.StartDate,
			"status":        orders.Status,
			"paymentStatus": orders.PaymentStatus,
			"message":       orders.Message,
			"_id":           orders.Id,
			"totalPrice":    orders.TotalPrice,
			"totalDiscount": orders.TotalDiscount,
			"totalQuantity": orders.TotalQuantity,
			"postalCost":    orders.PostalCost,
			"userId":        orders.UserId,
			"products":      orders.Products,
			"jStartDate":    orders.JStartDate,
			"address":       orders.Address,
			"createdAt":     orders.CreatedAt,
			"updatedAt":     orders.UpdatedAt,
			"__v":           orders.V,
		}

		// Send the response
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})
	} else if len(orders.Products) > 0 && len(orders.Mix) > 0 {
		var mixes []entities.Mixes
		var mixProductsMap = make(map[string][]entities.MixProducts) // Map to store mix products by mix ID
		for _, mixID := range orders.Mix {
			var mix entities.Mixes

			// Find the mix document by its ID
			err := mixesCollection.FindOne(c, bson.M{"_id": mixID}).Decode(&mix)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find mix", "details": err.Error()})
				return
			}
			mixes = append(mixes, mix)

			// Fetch mix products for the current mix
			var mixProducts []entities.MixProducts
			cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
				return
			}

			defer cur.Close(c)

			// Iterate through the cursor to decode each mix product
			for cur.Next(c) {
				var mixProduct entities.MixProducts
				if err := cur.Decode(&mixProduct); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				mixProducts = append(mixProducts, mixProduct)
			}
			if err := cur.Err(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Store mix products in the map with mix ID as the key (convert mix ID to string)
			mixProductsMap[mix.ID.Hex()] = mixProducts
		}

		// Construct mix array for the response body
		var mixArray []gin.H
		for _, mix := range mixes {
			mixProductArray, ok := mixProductsMap[mix.ID.Hex()] // Convert mix ID to string
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "mix products not found for mix ID: " + mix.ID.Hex()})
				return
			}
			mixDetails := gin.H{
				"_id":       mix.ID,
				"name":      mix.Name,
				"weight":    mix.Weight,
				"pattern":   mix.Pattern,
				"userId":    mix.UserId,
				"createdAt": mix.CreatedAt,
				"updatedAt": mix.UpdatedAt,
				"__v":       mix.V,
				"balance":   mix.Balance,
				"products":  mixProductArray,
			}
			mixArray = append(mixArray, mixDetails)
		}

		responseBody := gin.H{
			"isCoupon":      orders.IsCoupon,
			"startDate":     orders.StartDate,
			"status":        orders.Status,
			"paymentStatus": orders.PaymentStatus,
			"message":       orders.Message,
			"_id":           orders.Id,
			"totalPrice":    orders.TotalPrice,
			"totalDiscount": orders.TotalDiscount,
			"totalQuantity": orders.TotalQuantity,
			"postalCost":    orders.PostalCost,
			"userId":        orders.UserId,
			"products":      orders.Products,
			"jStartDate":    orders.JStartDate,
			"address":       orders.Address,
			"mixes":         mixArray,
			"createdAt":     orders.CreatedAt,
			"updatedAt":     orders.UpdatedAt,
			"__v":           orders.V,
		}

		// Send the response
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})

	}
}

// func GetOrderByID(c *gin.Context) {
// 	orderID := c.Param("id")

// 	// Convert orderID to int
// 	orderIntID, err := strconv.Atoi(orderID)
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
// 		return
// 	}

// 	tokenClaims, exists := c.Get("tokenClaims")
// 	if !exists {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
// 		return
// 	}

// 	claims, ok := tokenClaims.(*auth.SignedUserDetails)
// 	if !ok {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
// 		return
// 	}

// 	userID := claims.Id

// 	var orders entities.Order
// 	err = ordersCollection.FindOne(c, bson.M{"_id": orderIntID, "userId": userID}).Decode(&orders)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"messageOrder": err.Error()})
// 		return
// 	}

// 	// // Check if the order has a mix
// 	if len(orders.Products) == 0 {
// 		// Fetch the mix details
// 		var mix entities.Mixes
// 		err = mixesCollection.FindOne(c, bson.M{"_id": orders.Mix}).Decode(&mix)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"messageOrder": "mix not found"})
// 			return
// 		}

// 		// Fetch the mix products details
// 		var mixProducts []entities.MixProducts
// 		cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
// 			return
// 		}

// 		defer cur.Close(c)

// 		// Iterate through the cursor to decode each variation
// 		for cur.Next(c) {
// 			var mixProduct entities.MixProducts
// 			if err := cur.Decode(&mixProduct); err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 				return
// 			}

// 			mixProducts = append(mixProducts, mixProduct)
// 		}
// 		if err := cur.Err(); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		// Construct the response body including mix and mix products
// 		responseBody := gin.H{
// 			"isCoupon":      orders.IsCoupon,
// 			"startDate":     orders.StartDate,
// 			"status":        orders.Status,
// 			"paymentStatus": orders.PaymentStatus,
// 			"message":       orders.Message,
// 			"_id":           orders.Id,
// 			"totalPrice":    orders.TotalPrice,
// 			"totalDiscount": orders.TotalDiscount,
// 			"totalQuantity": orders.TotalQuantity,
// 			"postalCost":    orders.PostalCost,
// 			"userId":        orders.UserId,
// 			"products":      orders.Products,
// 			"jStartDate":    orders.JStartDate,
// 			"address":       orders.Address,
// 			"mix": gin.H{
// 				"products":  mixProducts,
// 				"_id":       mix.ID,
// 				"name":      mix.Name,
// 				"weight":    mix.Weight,
// 				"pattern":   mix.Pattern,
// 				"userId":    mix.UserId,
// 				"createdAt": mix.CreatedAt,
// 				"updatedAt": mix.UpdatedAt,
// 				"__v":       mix.V,
// 				"balance":   mix.Balance,
// 			},
// 			"createdAt": orders.CreatedAt,
// 			"updatedAt": orders.UpdatedAt,
// 			"__v":       orders.V,
// 		}

// 		// Send the response
// 		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})
// 	} else if orders.Mix.IsZero() {
// 		// If the order doesn't have a mix, handle it as usual
// 		// Construct the response body without mix details
// 		responseBody := gin.H{
// 			"isCoupon":      orders.IsCoupon,
// 			"startDate":     orders.StartDate,
// 			"status":        orders.Status,
// 			"paymentStatus": orders.PaymentStatus,
// 			"message":       orders.Message,
// 			"_id":           orders.Id,
// 			"totalPrice":    orders.TotalPrice,
// 			"totalDiscount": orders.TotalDiscount,
// 			"totalQuantity": orders.TotalQuantity,
// 			"postalCost":    orders.PostalCost,
// 			"userId":        orders.UserId,
// 			"products":      orders.Products,
// 			"jStartDate":    orders.JStartDate,
// 			"address":       orders.Address,
// 			"createdAt":     orders.CreatedAt,
// 			"updatedAt":     orders.UpdatedAt,
// 			"__v":           orders.V,
// 		}

// 		// Send the response
// 		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})
// 	} else if len(orders.Products) > 0 && !orders.Mix.IsZero() {
// 		// Fetch the mix details
// 		var mix entities.Mixes
// 		err = mixesCollection.FindOne(c, bson.M{"_id": orders.Mix}).Decode(&mix)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"messageOrder": "mix not found"})
// 			return
// 		}

// 		// Fetch the mix products details
// 		var mixProducts []entities.MixProducts
// 		cur, err := mixProductCollection.Find(c, bson.M{"_id": bson.M{"$in": mix.Products}})
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "not found mixproduct"})
// 			return
// 		}

// 		defer cur.Close(c)

// 		// Iterate through the cursor to decode each variation
// 		for cur.Next(c) {
// 			var mixProduct entities.MixProducts
// 			if err := cur.Decode(&mixProduct); err != nil {
// 				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 				return
// 			}

// 			mixProducts = append(mixProducts, mixProduct)
// 		}
// 		if err := cur.Err(); err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		// Construct the response body including mix and mix products
// 		responseBody := gin.H{
// 			"isCoupon":      orders.IsCoupon,
// 			"startDate":     orders.StartDate,
// 			"status":        orders.Status,
// 			"paymentStatus": orders.PaymentStatus,
// 			"message":       orders.Message,
// 			"_id":           orders.Id,
// 			"totalPrice":    orders.TotalPrice,
// 			"totalDiscount": orders.TotalDiscount,
// 			"totalQuantity": orders.TotalQuantity,
// 			"postalCost":    orders.PostalCost,
// 			"userId":        orders.UserId,
// 			"products":      orders.Products,
// 			"jStartDate":    orders.JStartDate,
// 			"address":       orders.Address,
// 			"mix": gin.H{
// 				"products":  mixProducts,
// 				"_id":       mix.ID,
// 				"name":      mix.Name,
// 				"weight":    mix.Weight,
// 				"pattern":   mix.Pattern,
// 				"userId":    mix.UserId,
// 				"createdAt": mix.CreatedAt,
// 				"updatedAt": mix.UpdatedAt,
// 				"__v":       mix.V,
// 				"balance":   mix.Balance,
// 			},
// 			"createdAt": orders.CreatedAt,
// 			"updatedAt": orders.UpdatedAt,
// 			"__v":       orders.V,
// 		}

// 		// Send the response
// 		c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": responseBody})

// 	}
// }

func GetAllOrders(c *gin.Context) {
	var orders int64
	count, err := ordersCollection.CountDocuments(c, bson.M{})
	if err != nil {
		// Handle error
		return
	}
	orders = count

	c.JSON(http.StatusOK, gin.H{"count": orders})
}

type OrderPayload struct {
	Id         interface{} `json:"_id"`
	TotalPrice int         `json:"totalPrice"`
}

func SendToZarinpal(c *gin.Context) {
	var orderPayload OrderPayload

	if err := c.ShouldBindJSON(&orderPayload); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	id := orderPayload.Id
	totalPrice := orderPayload.TotalPrice
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalln("error loading .env file")
	}
	merchantid := os.Getenv("ZARINPAL_MERCHANT_ID")

	merchantID := merchantid
	baseURL := os.Getenv("BASE_URL")
	callbackPath := "/api/users/orders/checkout/verify"
	callbackURL := baseURL + callbackPath
	amount := totalPrice
	description := "Payment for order ID"

	request := zarinpal.NewRequest(merchantID, callbackURL, uint(amount), description)

	requestResponse, err := request.Exec()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// fmt.Println("respons:", requestResponse)

	if requestResponse.Status == 100 {
		_, err := ordersCollection.UpdateOne(c,
			bson.M{"_id": id},
			bson.M{"$set": bson.M{"paymentId": requestResponse.Authority}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "pay", "body": gin.H{"url": "https://www.zarinpal.com/pg/StartPay/" + requestResponse.Authority}})

		return
	} else {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment request failed"})

		return
	}

}

func BackPayment(c *gin.Context) {
	var orderData entities.Order
	var user entities.Users
	authority := c.Query("Authority")
	// baseURL := os.Getenv("BASE_URL")

	err := ordersCollection.FindOne(c, bson.M{"paymentId": authority}).Decode(&orderData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}
	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalln("error loading .env file")
	}
	merchantid := os.Getenv("ZARINPAL_MERCHANT_ID")

	merchantID := merchantid

	amount := orderData.TotalPrice

	verify := zarinpal.NewVerify(merchantID, authority, uint(amount))
	verifyResponse, err := verify.Exec()

	if err != nil {
		c.HTML(http.StatusOK, "unsuccessful_payment.html", nil)
		return
	}

	orderIdStr := strconv.Itoa(int(orderData.Id.(int32)))

	if verifyResponse.Status == 100 || verifyResponse.Status == 101 {

		_, err := ordersCollection.UpdateOne(c,
			bson.M{"paymentId": authority},
			bson.M{"$set": bson.M{"paymentStatus": "paid"}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		err = usersCollection.FindOne(c, bson.M{"_id": orderData.UserId}).Decode(&user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"errorUser": err.Error()})
			return
		}
		// Get the raw phone number from the user document
		rawPhoneNumber := user.PhoneNumber

		// Prepend the leading zero to the phone number
		phoneNumberWithZero := "0" + rawPhoneNumber
		helpers.BackPaymentSendSms(orderIdStr, phoneNumberWithZero)

		c.HTML(http.StatusOK, "success_payment.html", nil)

		// c.JSON(http.StatusOK, gin.H{"status": "success"})
	} else {
		// c.Redirect(http.StatusNotFound, baseURL)
		_, err := ordersCollection.UpdateOne(c,
			bson.M{"paymentId": authority},
			bson.M{"$set": bson.M{"paymentStatus": "unpaid"}},
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, "unsuccessful_payment.html", nil)

	}
}

// func ServeFontandler(c *gin.Context) {
// 	c.File("./assets/font.css")
// 	c.Status(http.StatusNotModified)
// }
func ServeFontandler(c *gin.Context) {
	content, err := ioutil.ReadFile("./assets/font.css")
	if err != nil {
		// Handle error (e.g., log it, return an error response, etc.)
		return
	}
	c.Data(http.StatusOK, "text/css", content)
}
func ServeStyleCSSHandler(c *gin.Context) {
	content, err := ioutil.ReadFile("./assets/style.css")
	if err != nil {
		// Handle error (e.g., log it, return an error response, etc.)
		return
	}
	c.Data(http.StatusOK, "text/css", content)
}
func ServeImageCancel(c *gin.Context) {
	content, err := ioutil.ReadFile("./assets/cancel.png")
	if err != nil {
		// Handle error (e.g., log it, return an error response, etc.)
		return
	}
	c.Data(http.StatusOK, "text/css", content)
}
func ServeImageOk(c *gin.Context) {
	content, err := ioutil.ReadFile("./assets/checked.png")
	if err != nil {
		// Handle error (e.g., log it, return an error response, etc.)
		return
	}
	c.Data(http.StatusOK, "text/css", content)
}
