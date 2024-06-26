package service

import (
	"math"
	"net/http"
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

var commentCollection *mongo.Collection = database.GetCollection(database.DB, "comments")

var userCollection *mongo.Collection = database.GetCollection(database.DB, "users")

func GetCommentByAdmin(c *gin.Context) {
	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	_, ok := tokenClaims.(*auth.SignedAdminDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	limit := 15 // Number of comments per page

	// Calculate offset
	offset := int64((page - 1) * limit)
	limit64 := int64(limit)

	// Query MongoDB with pagination
	var comments []entities.Comments
	cur, err := commentCollection.Find(c, bson.M{}, &options.FindOptions{
		Limit: &limit64,
		Skip:  &offset,
		Sort:  bson.M{"createdAt": -1},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cur.Close(c)

	// Decode results
	for cur.Next(c) {
		var comment entities.Comments
		if err := cur.Decode(&comment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		comments = append(comments, comment)
	}
	var customComments []gin.H
	// fmt.Println("comments:", comments)

	for _, co := range comments {

		res, err := userCollection.Find(c, bson.M{"_id": co.UserId})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error1": err.Error()})
			return
		}
		defer res.Close(c)
		for res.Next(c) {
			var user entities.Users
			err := res.Decode(&user)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error2": err.Error()})
				return
			}
			customComment := gin.H{
				"id":        co.Id,
				"buyOffer":  co.BuyOffer,
				"isActive":  co.IsActive,
				"title":     co.Title,
				"text":      co.Text,
				"rate":      co.Rate,
				"productId": co.ProductId,
				"userId":    user,
				"createdAt": co.CreatedAt,
				"updatedAt": co.UpdatedAt,
				"__v":       co.V,
			}
			customComments = append(customComments, customComment)
			// users = append(users, user)
		}

	}

	// Count total documents for pagination info
	totalDocs, err := commentCollection.CountDocuments(c, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

	// Prepare pagination response
	hasNextPage := page < totalPages
	nextPage := page + 1
	hasPrevPage := page > 1
	prevPage := page - 1

	// Response
	response := gin.H{
		"docs":          customComments,
		"totalDocs":     totalDocs,
		"limit":         limit,
		"totalPages":    totalPages,
		"page":          page,
		"pagingCounter": offset + 1,
		"hasPrevPage":   hasPrevPage,
		"hasNextPage":   hasNextPage,
		"prevPage":      prevPage,
		"nextPage":      nextPage,
	}

	c.JSON(http.StatusOK, response)

}
func CommentGetProductByIDByAdmin(c *gin.Context) {
	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	_, ok := tokenClaims.(*auth.SignedAdminDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error1": "Invalid 'id' parameter"})
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'id' parameter"})
		return
	}

	// Define a filter to find the product based on productId
	filter := bson.M{"_id": objectID}

	// Perform the query to find the product in the product collection
	var product entities.Products
	err = proCollection.FindOne(c, filter).Decode(&product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error1": err.Error()})
		return
	}

	// If the product is found, return it as JSON response
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "product", "body": product})
}

func UpdateCommentByAdmin(c *gin.Context) {
	tokenClaims, exists := c.Get("tokenClaims")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token claims not found in context"})
		return
	}

	_, ok := tokenClaims.(*auth.SignedAdminDetails)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid token claims type"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error1": "Invalid 'id' parameter"})
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'id' parameter"})
		return
	}

	// Define a filter to find the product based on productId
	filter := bson.M{"_id": objectID}
	updateFields := make(map[string]interface{})

	// Extract fields to update from request JSON
	if err := c.ShouldBindJSON(&updateFields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set the updatedAt field to the current time
	updateFields["updatedAt"] = time.Now()

	// Construct the update query
	updateQuery := bson.M{"$set": updateFields}

	// page.CreatedAt = time.Now()
	// page.UpdatedAt = time.Now()

	// _, err = pagesCollection.UpdateOne(c, filter, bson.M{
	// 	"$set": bson.M{
	// 		"meta":      page.Meta,
	// 		"mode":      page.Mode,
	// 		"rows":      page.Rows,
	// 		"url":       page.Url,
	// 		"creayedAt": page.CreatedAt,
	// 		"updatedAt": page.UpdatedAt,
	// 		"__v":       page.V,
	// 	},
	// })
	_, err = commentCollection.UpdateOne(c, filter, updateQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error2": err.Error()})
		return
	}
	// var comment entities.Comments

	// comment.CreatedAt = time.Now()
	// comment.UpdatedAt = time.Now()
	// err = c.ShouldBindJSON(&comment)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	// _, err = commentCollection.UpdateOne(c, filter, bson.M{
	// 	"$set": bson.M{
	// 		// "_id":       comment.Id,
	// 		"buyOffer":  comment.BuyOffer,
	// 		"isActive":  comment.IsActive,
	// 		"title":     comment.Title,
	// 		"text":      comment.Text,
	// 		"rate":      comment.Rate,
	// 		"productId": comment.ProductId,
	// 		"userId":    comment.UserId,
	// 		"createdAt": comment.CreatedAt,
	// 		"updatedAt": comment.UpdatedAt,
	// 		"__v":       comment.V,
	// 	},
	// })
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error4": err.Error()})
	// 	return
	// }
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "comment_updated", "body": gin.H{}})

}
