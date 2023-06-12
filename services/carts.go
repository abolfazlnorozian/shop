package services

import (
	"net/http"
	"shop/db"
	"shop/entity"
	"shop/middleware"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var cartCollection *mongo.Collection = db.GetCollection(db.DB, "brands")
var prodCollection *mongo.Collection = db.GetCollection(db.DB, "products")

func AddCatrs(c *gin.Context) {
	var cart entity.Catrs

	tokenClaims, exists := c.Get("tokenClaims")

	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	claims, ok := tokenClaims.(*middleware.SignedUserDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	username := claims.Username

	if err := c.ShouldBindJSON(&cart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid JSON data"})
		return
	}
	cart.Id = primitive.NewObjectID()
	cart.Status = "active"
	cart.UserName = username

	cart.CreatedAt = time.Now()
	cart.UpdatedAt = time.Now()

	// Check if a document with the same username exists
	filter := bson.M{"username": username}
	var existingDoc entity.Catrs
	err := cartCollection.FindOne(c, filter).Decode(&existingDoc)
	if err == nil {
		// If an existing document found, append the new product to the Products array
		existingDoc.Products = append(existingDoc.Products, cart.Products[0])

		// Update the existing document in the database
		update := bson.M{"$set": bson.M{
			"products":  existingDoc.Products,
			"updatedAt": time.Now(),
			"quantity":  1,
		}}
		_, err = cartCollection.UpdateOne(c, filter, update)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": existingDoc})
		return
	}

	// If no existing document found, create a new one
	cart.Products = []entity.ComeProduct{cart.Products[0]}

	// Insert the new document into the database
	_, err = cartCollection.InsertOne(c, cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": cart})
}

func GetCarts(c *gin.Context) {
	var pro []entity.Products

	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	claims, ok := tokenClaims.(*middleware.SignedUserDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	username := claims.Username

	cur, err := cartCollection.Find(c, bson.M{"username": username})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch carts"})
		return
	}
	defer cur.Close(c)

	for cur.Next(c) {
		var cart entity.Catrs
		err := cur.Decode(&cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode cart"})
			return
		}

		for _, product := range cart.Products {
			productID := product.ProductId

			// Retrieve product data from "products" collection based on productID
			var retrievedProduct entity.Products
			err := prodCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product"})
				return
			}

			pro = append(pro, retrievedProduct)
		}
	}

	if len(pro) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Products not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"products": pro})
}
