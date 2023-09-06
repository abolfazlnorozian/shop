package router

import (
	"shop/auth"
	"shop/services"
	"shop/upload"

	"github.com/gin-gonic/gin"
)

func ProRouter(r *gin.RouterGroup) {
	pro := r.Group("/")
	userAuth := r.Group("/")
	adminAuth := r.Group("/")

	adminAuth.Use(auth.AdminAuthenticate())
	//userAuth.Use(auth.UserAuthenticate())
	adminAuth.GET("/allproduct", services.FindAllProducts)
	adminAuth.POST("/addproduct", services.AddProduct())
	userAuth.GET("/getProduct/:slug", services.GetProductBySlug)
	pro.GET("/products", services.GetProductsByOneField)
}

func CategoryRouter(r *gin.RouterGroup) {
	c := r.Group("/")
	ca := r.Group("/")
	ca.Use(auth.AdminAuthenticate())

	c.GET("/categories", services.FindAllCategories)
	ca.POST("/add", services.AddCategories)
}
func AdminRoutes(r *gin.RouterGroup) {
	u := r.Group("/")
	u.POST("/createdAdmin", services.RegisterAdmins)
	u.POST("/login", services.LoginAdmin)
}
func Uploader(r *gin.RouterGroup) {

	up := r.Group("/admin")
	up.Use(auth.AdminAuthenticate())
	up.POST("/upload", upload.Uploadpath)

	up.GET("/downloads", upload.FindAllImages)

}
func Downloader(r *gin.RouterGroup) {
	down := r.Group("/")
	down.Static("/uploads", "./public/images")

}
func UserRoute(r *gin.RouterGroup) {
	us := r.Group("/users")

	authUser := r.Group("/")
	authUser.Use(auth.UserAuthenticate())
	authAdmin := r.Group("/")
	authAdmin.Use(auth.AdminAuthenticate())

	us.GET("/auth/smsverification", services.RegisterUsers)
	us.POST("/auth/login", services.LoginUsers)
	authAdmin.GET("/users2", services.GetAllUsers)
	authUser.PUT("/updated", services.UpdatedUser)
	authUser.GET("/users", services.GetUserByToken)

}
func OrderRouter(r *gin.RouterGroup) {
	or := r.Group("/")
	ordr := r.Group("/")
	or.Use(auth.AdminAuthenticate())
	ordr.Use(auth.UserAuthenticate())

	or.GET("orders", services.FindordersByadmin)
	ordr.POST("addorder", services.AddOrder)
}
func CartRouter(r *gin.RouterGroup) {
	ca := r.Group("/users")
	ca.Use(auth.UserAuthenticate())

	ca.POST("addCart", services.AddCatrs)
	ca.GET("/carts", services.GetCarts)
	ca.DELETE("/deletedCart", services.DeleteCart)
}

func BrandRoute(r *gin.RouterGroup) {
	b := r.Group("/")
	b.GET("/brands", services.GetBrands)
}

func PageRoute(r *gin.RouterGroup) {
	b := r.Group("/")
	b.GET("/pages/index", services.GetPages)
}
