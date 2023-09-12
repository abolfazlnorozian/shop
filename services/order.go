package services

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"shop/auth"
	"shop/database"
	"shop/entities"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ordersCollection *mongo.Collection = database.GetCollection(database.DB, "orders")
var XCollection *mongo.Collection = database.GetCollection(database.DB, "users")
var produCollection *mongo.Collection = database.GetCollection(database.DB, "products")
var countersCollection *mongo.Collection = database.GetCollection(database.DB, "counters")
var addusersCollection *mongo.Collection = database.GetCollection(database.DB, "brandschemas")

func FindordersByadmin(c *gin.Context) {
	if err := auth.CheckUserType(c, "admin"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var orders []entities.Order
	defer cancel()

	results, err := ordersCollection.Find(ctx, bson.M{})
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

	c.JSON(http.StatusOK, gin.H{"message": orders})

}

func AddOrder(c *gin.Context) {
	var order entities.Order

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
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid order format"})
		return
	}

	counter := struct {
		NextID int `bson:"next_id"`
	}{}

	err := countersCollection.FindOneAndUpdate(
		c,
		bson.M{"_id": "order_counter"},
		bson.M{"$inc": bson.M{"next_id": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&counter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate ID"})
		return
	}

	order.Id = counter.NextID
	order.Address.Id = primitive.NewObjectID()

	order.StartDate = time.Now()
	order.Status = "none"
	order.PaymentId = ""
	order.UserId = id

	order.TotalDiscount = 0
	order.PostalCost = 0
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.V = 0

	var cart entities.Catrs
	err = XCollection.FindOne(c, bson.M{"username": username}).Decode(&cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	}
	// var address entities.Addr
	// if err := c.ShouldBindJSON(&address); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid address format"})
	// 	return
	// }

	// user := entities.Users{
	// 	// Existing fields...
	// 	Address: append(user.Address, &address),
	// }

	// // Update the user document in the collection
	// _, err = userCollection.UpdateOne(c, bson.M{"_id": id}, bson.M{"$set": user})
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
	// 	return
	// }

	// Iterate over products in the cart and add them to the order
	for _, product := range cart.Products {
		productID := product.ProductId
		variationKey := product.VariationsKey
		productQuantity := product.Quantity

		// Retrieve product data from "products" collection based on productID
		var retrievedProduct entities.Products
		err := produCollection.FindOne(c, bson.M{"_id": productID, "variations": bson.M{"$elemMatch": bson.M{"keys": variationKey}}}).Decode(&retrievedProduct)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product"})
			return
		}
		// Check if the retrieved product is empty (not found)
		if retrievedProduct.ID.IsZero() {
			continue // Skip this product and move to the next one
		}

		// Find the selected variation
		var selectedVariation entities.Variation
		for _, variation := range retrievedProduct.Variations {
			if reflect.DeepEqual(variation.Keys, variationKey) {
				selectedVariation = variation
				break
			}
		}

		// Check if a valid variation was found
		if selectedVariation.Id.IsZero() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Selected variation not found"})
			return
		}

		// Extract specific fields from retrievedProduct and create a new Product object
		orderProduct := entities.Product{
			Quantity:     productQuantity,
			Id:           retrievedProduct.ID,
			Name:         retrievedProduct.Name,
			Price:        retrievedProduct.Price,
			VariationKey: selectedVariation.Keys,
		}
		fmt.Printf("orderProduct: %+v\n", orderProduct)

		order.Products = append(order.Products, orderProduct)
		order.TotalQuantity += productQuantity
		order.TotalPrice += retrievedProduct.Price
	}

	// Remove the shopping cart from the "brandCollection"
	_, err = XCollection.DeleteOne(c, bson.M{"username": username})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove cart"})
		return
	}

	// Insert order into the "ordersCollection"
	_, err = ordersCollection.InsertOne(c, order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Send the response to Postman
	c.JSON(http.StatusOK, gin.H{"message": order})
}
