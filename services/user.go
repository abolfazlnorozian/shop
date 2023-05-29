package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"shop/db"
	"shop/entity"
	"shop/middleware"
	"shop/related"
	"shop/response"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var usersCollection *mongo.Collection = db.GetCollection(db.DB, "users")
var validates = validator.New()

func RegisterUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var user entity.Users

	if err := c.Bind(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "not found"})
		return
	}
	validationErr := validate.Struct(user)
	if validationErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
		return
	}

	newVerifycode := middleware.GenerateRandomCode(4)
	fmt.Println(newVerifycode)
	hashedCode, err := related.HashPassword(newVerifycode)
	fmt.Println(hashedCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verify code"})
		return
	}
	user.VerifyCode = hashedCode
	// Update the user document in the database
	update := bson.D{bson.E{Key: "$set", Value: bson.D{bson.E{Key: "verifyCode", Value: user.VerifyCode}}}}

	_, err = usersCollection.UpdateOne(ctx, bson.D{bson.E{Key: "phoneNumber", Value: user.PhoneNumber}}, update)
	if err != nil {
		log.Panic(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	count, err := usersCollection.CountDocuments(ctx, bson.M{"phoneNumber": user.PhoneNumber})
	if count > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "this PhoneNumber already exists"})
		return
	}

	if err != nil {
		log.Panic(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error occured while cheking for the email"})
		return
	}

	user.CreatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	user.UpdatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	user.Id = primitive.NewObjectID()
	user.Role = "user"
	user.Sex = 1
	resultInsertionNumber, insertErr := usersCollection.InsertOne(ctx, user)
	if insertErr != nil {
		msg := fmt.Sprintf("User item was not created")
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, resultInsertionNumber)
}

func LoginUsers(c *gin.Context) {
	var ctx, cancle = context.WithTimeout(context.Background(), 100*time.Second)
	defer cancle()

	var user entity.Users
	var foundUser entity.Users

	if err := c.Bind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	err := usersCollection.FindOne(ctx, bson.M{"phoneNumber": user.PhoneNumber}).Decode(&foundUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	passwordIsValid, _ := related.VerifyPassword(user.VerifyCode, foundUser.VerifyCode)
	defer cancle()
	if passwordIsValid != true {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password is incorrect"})
		return
	}
	if foundUser.VerifyCode == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
	}

	token, refreshToken, _ := middleware.GenerateUserAllTokens(foundUser.Username, foundUser.PhoneNumber, foundUser.Role, foundUser.LastName, foundUser.Name)
	middleware.UpdateUserAllTokens(token, refreshToken, foundUser.Role)
	err = usersCollection.FindOne(ctx, bson.M{"role": foundUser.Role}).Decode(&foundUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "login", "body": gin.H{"token": &token, "refreshToken": &refreshToken}})

}

func GetAllUsers(c *gin.Context) {
	if err := middleware.CheckUserType(c, "admin"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var users []entity.Users
	defer cancel()

	results, err := usersCollection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"massage": "Not Find Collection"})
		return
	}
	//results.Close(ctx)
	for results.Next(ctx) {
		var user entity.Users
		err := results.Decode(&user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return

		}
		users = append(users, user)

	}

	c.JSON(http.StatusOK, response.Response{Status: http.StatusOK, Message: "success", Data: map[string]interface{}{"data": &users}})
}

func UpdatedUser(c *gin.Context) {
	var user entity.Users
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"massage": err.Error()})
		return
	}
	filter := bson.D{bson.E{Key: "phoneNumber", Value: user.PhoneNumber}}
	update := bson.D{bson.E{Key: "$set", Value: bson.D{
		bson.E{Key: "activeSession", Value: user.ActiveSession},
		bson.E{Key: "fcmRegistrationToken", Value: user.FcmRegistratinToken},
		bson.E{Key: "favoritesProducts", Value: user.Favorites},
		bson.E{Key: "username", Value: user.Username},
		bson.E{Key: "verifyCode", Value: user.VerifyCode},
		bson.E{Key: "phoneNumber", Value: user.PhoneNumber},
		bson.E{Key: "sex", Value: user.Sex},
		bson.E{Key: "role", Value: user.Role},
		bson.E{Key: "address", Value: user.Address},
		bson.E{Key: "__v", Value: user.V},
		bson.E{Key: "countGetSmsInDay", Value: user.CountGetSmsInDay},
		bson.E{Key: "lastname", Value: user.LastName},
		bson.E{Key: "name", Value: user.Name},
		bson.E{Key: "updatedAt", Value: time.Now()},
		bson.E{Key: "LastSendSmsVerificationTime", Value: time.Now()},
	}}}

	result, err := usersCollection.UpdateOne(c, filter, update)

	if result.ModifiedCount == 0 {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"massage": "Updated"})

}
