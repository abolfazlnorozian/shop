package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Order struct {
	Id                interface{}          `json:"_id" bson:"_id,omitempty"`
	IsCoupon          bool                 `json:"isCoupon" bson:"isCoupon"`
	StartDate         time.Time            `json:"startDate" bson:"startDate"`
	Status            string               `json:"status" bson:"status"`
	PaymentStatus     string               `json:"paymentStatus" bson:"paymentStatus"`
	Message           string               `json:"message" bson:"message"`
	TotalPrice        int                  `json:"totalPrice" bson:"totalPrice"`
	TotalDiscount     float64              `json:"totalDiscount" bson:"totalDiscount"`
	AmountCoupon      int                  `json:"amountCoupon" bson:"amountCoupon"`
	CouponCode        string               `json:"couponCode" bson:"couponCode"`
	TotalQuantity     int                  `json:"totalQuantity" bson:"totalQuantity"`
	PostalCost        int                  `json:"postalCost" bson:"postalCost"`
	UserId            primitive.ObjectID   `json:"userId" bson:"userId"`
	Products          []Product            `json:"products" bson:"products"`
	JStartDate        string               `json:"jStartDate" bson:"jStartDate"`
	Address           Addrs                `json:"address" bson:"address" binding:"required"`
	Mix               []primitive.ObjectID `json:"mix,omitempty" bson:"mix,omitempty"`
	CreatedAt         time.Time            `json:"createdAt" bson:"createdAt"`
	UpdatedAt         time.Time            `json:"updatedAt" bson:"updatedAt"`
	V                 int                  `json:"__v" bson:"__v"`
	PaymentId         string               `json:"paymentId" bson:"paymentId"`
	PostalTrakingCode string               `json:"postalTrakingCode" bson:"postalTrakingCode"`
}

type Product struct {
	Quantity        int                `json:"quantity" bson:"quantity"`
	VariationKey    []int              `json:"variationsKey" bson:"variationsKey"`
	Id              primitive.ObjectID `json:"_id" bson:"_id"`
	Name            string             `json:"name" bson:"name"`
	Price           int                `json:"price" bson:"price"`
	DiscountPercent float64            `json:"discountPercent" bson:"discountPercent"`
	ProductId       primitive.ObjectID `json:"productId" bson:"productId"`
}
type Addrs struct {
	Id      primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Address string             `json:"address" bson:"address" binding:"required"`
	City    string             `json:"city" bson:"city" binding:"required"`
	// Latitude   float64            `json:"latitude" bson:"latitude"`
	// Longitude  float64            `json:"longitude" bson:"longitude"`
	PostalCode interface{} `json:"postalCode" bson:"postalCode" binding:"required"`
	State      string      `json:"state" bson:"state" binding:"required"`
}

// type Order struct {
// 	Id                interface{}          `json:"_id" bson:"_id,omitempty"`
// 	IsCoupon          bool                 `json:"isCoupon" bson:"isCoupon"`
// 	StartDate         time.Time            `json:"startDate" bson:"startDate"`
// 	Status            string               `json:"status" bson:"status"`
// 	PaymentStatus     string               `json:"paymentStatus" bson:"paymentStatus"`
// 	Message           string               `json:"message" bson:"message"`
// 	TotalPrice        int                  `json:"totalPrice" bson:"totalPrice"`
// 	TotalDiscount     float64              `json:"totalDiscount" bson:"totalDiscount"`
// 	AmountCoupon      int                  `json:"amountCoupon" bson:"amountCoupon"`
// 	CouponCode        string               `json:"couponCode" bson:"couponCode"`
// 	TotalQuantity     int                  `json:"totalQuantity" bson:"totalQuantity"`
// 	PostalCost        int                  `json:"postalCost" bson:"postalCost"`
// 	UserId            primitive.ObjectID   `json:"userId" bson:"userId"`
// 	Products          []Product            `json:"products" bson:"products"`
// 	JStartDate        string               `json:"jStartDate" bson:"jStartDate"`
// 	Address           Addrs                `json:"address" bson:"address" binding:"required"`
// 	Mix               []primitive.ObjectID `json:"mix,omitempty" bson:"mix,omitempty"`
// 	CreatedAt         time.Time            `json:"createdAt" bson:"createdAt"`
// 	UpdatedAt         time.Time            `json:"updatedAt" bson:"updatedAt"`
// 	V                 int                  `json:"__v" bson:"__v"`
// 	PaymentId         string               `json:"paymentId" bson:"paymentId"`
// 	PostalTrakingCode string               `json:"postalTrakingCode" bson:"postalTrakingCode"`
// }

// type ResponseProduct struct {
// 	Quantity        int                `json:"quantity" bson:"quantity"`
// 	VariationKey    []int              `json:"variationsKey" bson:"variationsKey"`
// 	Id              primitive.ObjectID `json:"_id" bson:"_id"`
// 	Name            string             `json:"name" bson:"name"`
// 	Price           int                `json:"price" bson:"price"`
// 	DiscountPercent float64            `json:"discountPercent" bson:"discountPercent"`
// 	ProductId       primitive.ObjectID `json:"productId" bson:"productId"`
// }
