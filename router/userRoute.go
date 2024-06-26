// Package router provides ...
// @title Shop API
// @version 1.0
// @description API for the Shop application
// @host localhost:3006
// @BasePath /api

package router

import (
	"shop/auth"
	"shop/user/services"

	"github.com/gin-gonic/gin"
)

func ProRouter(r *gin.RouterGroup) {
	pro := r.Group("/")

	userAuth := r.Group("/")
	userAuth.Use(auth.UserAuthenticate)

	// adminAuth.POST("/addproduct", services.AddProduct())
	// adminAuth.GET("/products", service.GetAllProductsByAdmin)

	pro.GET("/products", services.GetProductsByFields)
	pro.GET("/products/", services.GetProductByCategory)
	pro.GET("/products/:slug", services.GetProductBySlug)
	pro.GET("/mix-products", services.GetMixProducts)
	userAuth.POST("/mix", services.PostMixesProduct)
	userAuth.OPTIONS("/mix", services.PostMixesProduct)
	userAuth.DELETE("/mix/:id", services.DeleteMixofCart)
	userAuth.OPTIONS("/mix/:id", services.DeleteMixofCart)
	pro.GET("/products/count", services.GetAllCountProducts)

}

// func OptionsMixofCart(c *gin.Context) {
// 	c.Status(http.StatusOK)
// }

//userAuth.Use(auth.UserAuthenticate())

func CategoryRouter(r *gin.RouterGroup) {
	c := r.Group("/")

	c.GET("/categories", services.FindAllCategories)
	// c.OPTIONS("/categories", services.FindAllCategories)
	// ca.POST("/add", services.AddCategories)
	c.GET("/categories/:slug", services.GetOneGategory)
	c.GET("/categories/undefined", services.UndefindProduct)
}

// func Uploader(r *gin.RouterGroup) {

// 	up := r.Group("/admin")
// 	up.Use(auth.AdminAuthenticate())

// 	up.GET("/downloads", upload.FindAllImages)

// }
func Downloader(r *gin.RouterGroup) {
	down := r.Group("/")
	down.Static("/uploads", "./public/images")
	//down.GET("/uploads/:filename", upload.FindOneImage)

}

func UserRoute(r *gin.RouterGroup) {
	us := r.Group("/users")

	authUser := r.Group("/")
	authUser.Use(auth.UserAuthenticate)
	authAdmin := r.Group("/")
	authAdmin.Use(auth.AdminAuthenticate())

	us.GET("/auth/smsverification", services.RegisterUsers)
	us.POST("/auth/login", services.LoginUsers)
	us.OPTIONS("/auth/login", services.LoginUsers)
	authAdmin.GET("/users2", services.GetAllUsers)
	authUser.GET("/users", services.GetUserByToken)
	authUser.OPTIONS("/users", services.GetUserByToken)
	authUser.PUT("/users/", services.UpdatedUser)
	authUser.OPTIONS("/users/", services.UpdatedUser)

	authUser.POST("/users/addresses", services.PostAddresses)
	authUser.GET("/users/addresses", services.GetAddresses)
	authUser.OPTIONS("/users/addresses", services.GetAddresses)
	authUser.DELETE("/users/addresses/:id", services.DeleteAddressByID)
	authUser.OPTIONS("/users/addresses/:id", services.DeleteAddressByID)
	us.GET("/count", services.GetAllCountUsers)

}
func OrderRouter(r *gin.RouterGroup) {
	or := r.Group("/")
	ordr := r.Group("/users")

	// or.Use(auth.AdminAuthenticate())
	ordr.Use(auth.UserAuthenticate)

	ordr.GET("/orders", services.Findorders)
	ordr.POST("/orders", services.AddOrder)
	ordr.OPTIONS("/orders", services.AddOrder)
	ordr.GET("/orders/:id", services.GetOrderByID)
	ordr.OPTIONS("/orders/:id", services.GetOrderByID)
	ordr.POST("/orders/checkout", services.SendToZarinpal)
	ordr.OPTIONS("/orders/checkout", services.SendToZarinpal)
	// ordr.OPTIONS("/orders/checkout", services.OptionsOrers)

	or.GET("/users/orders/checkout/verify", services.BackPayment)
	or.GET("/users/orders/count", services.GetAllOrders)

}

func AssetsRoute(r *gin.RouterGroup) {
	html := r.Group("/")
	html.GET("/stylesheets/style.css", services.ServeStyleCSSHandler)
	html.GET("/stylesheets/fonts.css", services.ServeFontandler)
	html.GET("/images/cancel.png", services.ServeImageCancel)
	html.GET("/images/checked.png", services.ServeImageOk)

	// html.GET("/ipg/assets/style.css", services.ServeStyleCSSHandler)
	// html.GET("/ipg/assets/font.css", services.ServeFontandler)
	// html.GET("/ipg/assets/cancel.png", services.ServeImageHandler)
}
func CartRouter(r *gin.RouterGroup) {
	ca := r.Group("/users")
	ca.Use(auth.UserAuthenticate)
	mix := r.Group("/")
	mix.Use(auth.UserAuthenticate)

	ca.POST("/carts/", services.AddCatrs)
	ca.OPTIONS("/carts/", services.AddCatrs)
	ca.OPTIONS("/carts", services.OptionsCarts)
	ca.GET("/carts", services.GetCarts)
	//ca.OPTIONS("/carts", services.GetCarts)

	// ca.DELETE("/carts/:id", services.DeleteCart)
	ca.DELETE("/carts", services.DeleteProductFromCart)

}

// func OptionsMixofCart(c *gin.Context) {
// 	c.Status(http.StatusOK)
// }

func State(r *gin.RouterGroup) {
	s := r.Group("/json")

	s.GET("/state.json", services.State)

}

func BrandRoute(r *gin.RouterGroup) {
	b := r.Group("/")
	b.GET("/brands", services.GetBrands)
}

func PageRoute(r *gin.RouterGroup) {
	b := r.Group("/")
	b.GET("/pages/index", services.GetPages)
}

func CommentRoute(r *gin.RouterGroup) {
	com := r.Group("/")
	c := r.Group("/")
	c.Use(auth.UserAuthenticate)
	c.POST("/products/:productID/comments", services.PostComment)
	c.OPTIONS("/products/:productID/comments", services.PostComment)
	com.GET("/products/:slug/comments", services.GetComment)
}
func FavoriteRoute(r *gin.RouterGroup) {
	b := r.Group("/")
	b.Use(auth.UserAuthenticate)
	b.POST("/users/favorites", services.AddProductToFavorite)
	b.OPTIONS("users/favorites", services.AddProductToFavorite)
	b.GET("/users/favorites", services.GetFavorites)
	b.OPTIONS("users/favorites/:productID", services.DeleteFavorites)

	b.DELETE("/users/favorites/:productID", services.DeleteFavorites)
}
func ApiGh(r *gin.RouterGroup) {
	a := r.Group("/")
	a.GET("/", services.ApiGhahvedark)
}

//****************************************************************************************************
