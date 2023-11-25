package services

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"shop/auth"
	"shop/database"
	"shop/entities"
	"strconv"
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
var countersCollection *mongo.Collection = database.GetCollection(database.DB, "identitycounters")
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

	c.JSON(http.StatusCreated, gin.H{"message": orders})

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

	var users entities.Users
	err := usersCollection.FindOne(c, bson.M{"username": username}).Decode(&users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"errorAddress": err.Error()})
		return
	}
	for _, user := range users.Address {
		objectID := user.Id

		// Initialize the order.Address with a new object
		order.Address = entities.Addrs{
			Id:         objectID,
			Address:    user.Address,
			City:       user.City,
			PostalCode: user.PostalCode,
			State:      user.State,
		}

	}

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
	order.PaymentId = ""
	order.UserId = id
	order.IsCoupon = false
	order.Massage = ""

	order.TotalDiscount = 0
	order.PostalCost = 0
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.V = 0

	var cart entities.Catrs
	err = cartCollection.FindOne(c, bson.M{"username": username}).Decode(&cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "not found this user in cart"})
		return

	}

	// Iterate over products in the cart and add them to the order
	for _, product := range cart.Products {

		productID := product.ProductId
		variationKey := product.VariationsKey
		productQuantity := product.Quantity
		fmt.Println("productId:", productID)
		// Retrieve product data from "products" collection based on productID
		var retrievedProduct entities.Products
		// err := produCollection.FindOne(c, bson.M{"_id": productID, "variations": bson.M{"$elemMatch": bson.M{"keys": variationKey}}}).Decode(&retrievedProduct)
		err := proCollection.FindOne(c, bson.M{"_id": productID}).Decode(&retrievedProduct)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"errorProduct": err.Error()})
			return
		}
		// fmt.Println("reteriveProduct:", retrievedProduct)
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

		// Extract specific fields from retrievedProduct and create a new Product object
		orderProduct := entities.Product{
			Quantity:     productQuantity,
			Id:           retrievedProduct.ID,
			Name:         retrievedProduct.Name,
			Price:        retrievedProduct.Price,
			VariationKey: selectedVariation.Keys,
			ProductId:    product.ProductId,
		}

		order.Products = append(order.Products, orderProduct)
		order.TotalQuantity += productQuantity
		order.TotalPrice += retrievedProduct.Price
	}

	// Remove the shopping cart from the "brandCollection"
	_, err = cartCollection.DeleteOne(c, bson.M{"username": username})
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
	c.JSON(http.StatusOK, gin.H{"message": "order_added", "success": true, "body": gin.H{"orderId": order.Id}})
}

func GetOrder(c *gin.Context) {
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
	order := entities.Order{
		Id:            orders.Id,
		IsCoupon:      orders.IsCoupon,
		StartDate:     orders.StartDate,
		Status:        orders.Status,
		PaymentStatus: orders.PaymentStatus,
		Massage:       orders.Massage,
		TotalPrice:    orders.TotalPrice,
		TotalDiscount: orders.TotalDiscount,
		TotalQuantity: orders.TotalQuantity,
		PostalCost:    orders.PostalCost,
		UserId:        orders.UserId,
		Products:      orders.Products,
		JStartDate:    orders.JStartDate,
		Address:       orders.Address,
		CreatedAt:     orders.CreatedAt,
		UpdatedAt:     orders.UpdatedAt,
		V:             orders.V,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "order", "body": order})
}
