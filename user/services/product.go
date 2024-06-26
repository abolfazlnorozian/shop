package services

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"shop/auth"
	"shop/database"
	"shop/entities"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var proCollection *mongo.Collection = database.GetCollection(database.DB, "products")
var propertiesCollection *mongo.Collection = database.GetCollection(database.DB, "properties")
var mixProductCollection *mongo.Collection = database.GetCollection(database.DB, "mixproducts")
var mixesCollection *mongo.Collection = database.GetCollection(database.DB, "mixes")

type ProductWithCategories struct {
	entities.Products

	Categories []entities.Category `json:"categories"`
	Dimension  []DimensionResponse `json:"dimensions"`
}

type DimensionResponse struct {
	Key    entities.Properties   `json:"key"`
	Values []entities.Properties `json:"values"`
	ID     primitive.ObjectID    `json:"_id,omitempty" bson:"_id,omitempty"`
}

// ProRouter provides ...
// @Summary Get products by slug
// @Description Get products by slug
// @Tags products
// @Accept json
// @Produce json
// @Param slug path string true "Product Slug"
// @Success 200 {object} services.ProductWithCategories
// @Router /api/products/{slug} [get]
func GetProductBySlug(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slug := c.Param("slug")
	var proWithCategories ProductWithCategories

	err := proCollection.FindOne(ctx, bson.M{"slug": slug}).Decode(&proWithCategories.Products)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch category details and store them in the Categories field
	categories, err := fetchCategoryDetails(ctx, proWithCategories.Category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Assign the fetched categories to the Categories field
	proWithCategories.Categories = categories

	var pro []entities.Dimension

	for _, value := range proWithCategories.Dimensions {
		pro = append(pro, entities.Dimension{
			Key:    value.Key,
			Values: value.Values,
			ID:     value.ID,
		})
	}

	dimensions, err := fetchPropertyDetails(ctx, pro)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	proWithCategories.Dimension = dimensions
	// Sort variations by quantity (1 first, then 0)
	sort.Slice(proWithCategories.Variations, func(i, j int) bool {
		return proWithCategories.Variations[i].Quantity > proWithCategories.Variations[j].Quantity
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "product", "body": proWithCategories})
}
func fetchPropertyDetails(ctx context.Context, propertyIDs []entities.Dimension) ([]DimensionResponse, error) {
	var dimensionResponses []DimensionResponse

	for _, v := range propertyIDs {
		// Fetch key document
		var keyDocument entities.Properties
		err := propertiesCollection.FindOne(ctx, bson.M{"_id": v.Key}).Decode(&keyDocument)
		if err != nil {
			return nil, err
		}

		// Fetch values documents
		cursor, err := propertiesCollection.Find(ctx, bson.M{"_id": bson.M{"$in": v.Values}})
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		var values []entities.Properties
		for cursor.Next(ctx) {
			var property entities.Properties
			if err := cursor.Decode(&property); err != nil {
				return nil, err
			}
			values = append(values, property)
		}

		// Create DimensionResponse
		dimensionResponse := DimensionResponse{
			Key:    keyDocument,
			Values: values,
			ID:     v.ID,
		}

		dimensionResponses = append(dimensionResponses, dimensionResponse)
	}

	return dimensionResponses, nil
}

// Function to fetch category details from the category collection
func fetchCategoryDetails(ctx context.Context, categoryIDs []primitive.ObjectID) ([]entities.Category, error) {
	var categories []entities.Category

	cursor, err := categoryCollection.Find(ctx, bson.M{"_id": bson.M{"$in": categoryIDs}})
	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var category entities.Category
		if err := cursor.Decode(&category); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

//*****************************************************************************
// ProRouter provides ...
// @Summary Get products based on various filters
// @Description Get products based on category ID, category name, search query, amazing status, onlyExists status, or new status with optional pagination
// @Tags products
// @Accept json
// @Produce json
// @Param categoryid query string false "Category ID"
// @Param category query string false "Category Name"
// @Param search query string false "Search query for fuzzy matching product names"
// @Param amazing query string false "Filter by amazing products (true/false)"
// @Param onlyexists query string false "Filter by only existing products (true/false)"
// @Param new query string false "Filter by new products (1 for true, 0 for false)"
// @Param page query integer false "Page number for pagination (default is 1)"
// @Param limit query integer false "Number of items per page (default is 40)"
// @Success 200 {object} response.GetProductByOneField
// @Router /api/products [get]
func GetProductsByFields(c *gin.Context) {
	categoryId := c.DefaultQuery("categoryid", "")
	categoryName := c.DefaultQuery("category", "")
	searchQuery := c.DefaultQuery("search", "")
	amazingQuery := c.DefaultQuery("amazing", "") == "true"
	onlyExistsParam := c.DefaultQuery("onlyexists", "")
	isNewParam := c.DefaultQuery("new", "")

	switch {
	case categoryId != "":
		GetProductsByCategoryId(c)
	case categoryName != "":

		GetProductByCategory(c)
	case searchQuery != "":
		GetSearch(c)
	case amazingQuery != false:
		GetProductsByAmazing(c)
	case onlyExistsParam != "":
		GetProductsByOnlyExists(c)
	case isNewParam != "":
		GetProductsByOnlyExists(c)
	default:

		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid parameters"})
	}
}

//*****************************************************************************
func GetSearch(c *gin.Context) {
	filter := bson.M{}
	projection := bson.M{"name_fuzzy": 0}

	searchQuery := c.DefaultQuery("search", "")
	if searchQuery != "" {

		keywords := strings.Fields(searchQuery)

		regexPatterns := make([]bson.M, len(keywords))
		for i, keyword := range keywords {
			regexPatterns[i] = bson.M{"name_fuzzy": bson.M{"$regex": keyword, "$options": "i"}}
		}

		filter["$and"] = regexPatterns
	}

	var products []entities.Products
	cur, err := proCollection.Find(c, filter, options.Find().SetProjection(projection))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}
	defer cur.Close(c)
	for cur.Next(c) {
		var pro entities.Products
		err := cur.Decode(&pro)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}
		products = append(products, pro)
	}
	for _, p := range products {
		fmt.Println(p.ID)
	}
	// return products,nil

	c.JSON(http.StatusOK, gin.H{"pages": 1, "docs": products})
}

func GetProductsByAmazing(c *gin.Context) {

	filter := bson.M{}

	if c.DefaultQuery("amazing", "") == "true" {

		filter["amazing"] = true

	}
	// Pagination parameters from the query
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate skip value for pagination
	skip := (page - 1) * limit

	// Calculate total number of documents in the collection
	totalDocs, err := proCollection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}

	// Fetch products based on the constructed filter
	results, err := proCollection.Find(ctx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query products"})
		return
	}

	defer results.Close(ctx)
	var products []entities.Products

	for results.Next(ctx) {
		var pro entities.Products
		err := results.Decode(&pro)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		products = append(products, pro)
	}

	// Calculate total number of pages based on the limit
	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

	// Determine if there are previous and next pages
	hasPrevPage := page > 1
	hasNextPage := page < totalPages

	// Prepare the custom response with selected fields
	var customProducts []gin.H
	for _, product := range products {
		customProduct := gin.H{
			"_id":             product.ID,
			"notExist":        product.NotExist,
			"amazing":         product.Amazing,
			"productType":     product.ProductType,
			"images":          product.Images,
			"name":            product.Name,
			"price":           product.Price,
			"discountPercent": product.DiscountPercent,
			"stock":           product.Stock,
			"slug":            product.Slug,
			"variations":      product.Variations,
			"salesNumber":     product.SalesNumber,
			"bannerUrl":       product.BannerUrl,
		}
		customProducts = append(customProducts, customProduct)
	}

	// Prepare the response with custom products and pagination information
	response := gin.H{
		"docs":          customProducts,
		"totalDocs":     totalDocs,
		"limit":         limit,
		"totalPages":    totalPages,
		"page":          page,
		"pagingCounter": skip + 1,
		"hasPrevPage":   hasPrevPage,
		"hasNextPage":   hasNextPage,
	}

	// Set prevPage and nextPage values based on the current page
	if hasPrevPage {
		response["prevPage"] = page - 1
	} else {
		response["prevPage"] = nil
	}

	if hasNextPage {
		response["nextPage"] = page + 1
	} else {
		response["nextPage"] = nil
	}

	c.JSON(http.StatusOK, response)

}

//****************************************************************************************************

func GetProductsByCategoryId(c *gin.Context) {

	filter := bson.M{}
	categoryId := c.DefaultQuery("categoryid", "")
	if categoryId != "" {
		// Convert the categoryId string to ObjectID
		objectID, err := primitive.ObjectIDFromHex(categoryId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid Object ID"})
			return
		}

		var category entities.Category
		err = categoryCollection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&category)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Category not found"})
			return
		}

		categoryIDs, err := searchChildrenIDs(*category.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		}

		// fmt.Println(categoryIDs)
		if categoryIDs != nil {
			var categoryID []string
			for _, catID := range categoryIDs {

				categoryID = append(categoryID, catID.ID.Hex())
			}

			filter["categoryId"] = bson.M{"$in": categoryID}

		} else {
			filter["categoryId"] = category.ID.Hex()
		}
	}

	// Pagination parameters from the query
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate skip value for pagination
	skip := (page - 1) * limit

	// Calculate total number of documents in the collection
	totalDocs, err := proCollection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}
	// fmt.Println("totalDocts:", totalDocs)
	// Fetch products based on the constructed filter
	results, err := proCollection.Find(ctx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query products"})
		return
	}

	defer results.Close(ctx)

	var products []entities.Products

	for results.Next(ctx) {
		var pro entities.Products
		err := results.Decode(&pro)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		products = append(products, pro)
	}

	// Calculate total number of pages based on the limit
	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

	// Determine if there are previous and next pages
	hasPrevPage := page > 1
	hasNextPage := page < totalPages

	// Prepare the custom response with selected fields
	var customProducts []gin.H
	for _, product := range products {
		customProduct := gin.H{
			"_id":             product.ID,
			"notExist":        product.NotExist,
			"amazing":         product.Amazing,
			"productType":     product.ProductType,
			"images":          product.Images,
			"name":            product.Name,
			"price":           product.Price,
			"discountPercent": product.DiscountPercent,
			"stock":           product.Stock,
			"slug":            product.Slug,
			"variations":      product.Variations,
			"salesNumber":     product.SalesNumber,
			"bannerUrl":       product.BannerUrl,
		}
		customProducts = append(customProducts, customProduct)
	}

	// Prepare the response with custom products and pagination information
	response := gin.H{
		"docs":          customProducts,
		"totalDocs":     totalDocs,
		"limit":         limit,
		"totalPages":    totalPages,
		"page":          page,
		"pagingCounter": skip + 1,
		"hasPrevPage":   hasPrevPage,
		"hasNextPage":   hasNextPage,
	}

	// Set prevPage and nextPage values based on the current page
	if hasPrevPage {
		response["prevPage"] = page - 1
	} else {
		response["prevPage"] = nil
	}

	if hasNextPage {
		response["nextPage"] = page + 1
	} else {
		response["nextPage"] = nil
	}

	c.JSON(http.StatusOK, response)

}

//************************************************************************************************

func GetProductsByOnlyExists(c *gin.Context) {

	filter := bson.M{}

	onlyExistsParam := c.DefaultQuery("onlyexists", "")
	isNewParam := c.DefaultQuery("new", "")

	if onlyExistsParam == "true" {

		filter = bson.M{}
	}

	if onlyExistsParam == "true" && isNewParam == "1" {

		filter = bson.M{}
	}

	// Pagination parameters from the query
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate skip value for pagination
	skip := (page - 1) * limit

	// Calculate total number of documents in the collection
	totalDocs, err := proCollection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}
	// results, err := proCollection.Find(ctx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query products"})
	// 	return
	// }

	// defer results.Close(ctx)
	// var products []entities.Products

	// for results.Next(ctx) {
	// 	var pro entities.Products
	// 	err := results.Decode(&pro)
	// 	if err != nil {
	// 		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
	// 		return
	// 	}

	// 	products = append(products, pro)
	// }

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "updatedAt", Value: -1}}).SetLimit(50)
	cursor, err := proCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}
	defer cursor.Close(ctx)

	var products []entities.Products
	for cursor.Next(ctx) {
		var product entities.Products
		if err := cursor.Decode(&product); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
			return
		}
		products = append(products, product)
	}
	if err := cursor.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}

	// Calculate total number of pages based on the limit
	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

	// Determine if there are previous and next pages
	hasPrevPage := page > 1
	hasNextPage := page < totalPages

	// Prepare the custom response with selected fields
	var customProducts []gin.H
	for _, product := range products {
		customProduct := gin.H{
			"_id":             product.ID,
			"notExist":        product.NotExist,
			"amazing":         product.Amazing,
			"productType":     product.ProductType,
			"images":          product.Images,
			"name":            product.Name,
			"price":           product.Price,
			"discountPercent": product.DiscountPercent,
			"stock":           product.Stock,
			"slug":            product.Slug,
			"variations":      product.Variations,
			"salesNumber":     product.SalesNumber,
			"bannerUrl":       product.BannerUrl,
		}
		customProducts = append(customProducts, customProduct)
	}

	// Prepare the response with custom products and pagination information
	response := gin.H{
		"docs":          customProducts,
		"totalDocs":     totalDocs,
		"limit":         limit,
		"totalPages":    totalPages,
		"page":          page,
		"pagingCounter": skip + 1,
		"hasPrevPage":   hasPrevPage,
		"hasNextPage":   hasNextPage,
	}

	// Set prevPage and nextPage values based on the current page
	if hasPrevPage {
		response["prevPage"] = page - 1
	} else {
		response["prevPage"] = nil
	}

	if hasNextPage {
		response["nextPage"] = page + 1
	} else {
		response["nextPage"] = nil
	}

	c.JSON(http.StatusOK, response)

}

//***********************************************************************
// ProRouter provides ...
// @Summary Get products by category
// @Description Get products by category with optional pagination
// @Tags products
// @Accept json
// @Produce json
// @Param category query string false "categoryName or slug"
// @Param page query integer false "Page number for pagination (default is 1)"
// @Param limit query integer false "Number of items per page (default is 40)"
// @Success 200 {object} response.GetProductByOneField
// @Router /api/products/ [get]
func GetProductByCategory(c *gin.Context) {
	//*********************
	// Set a default filter to fetch all products
	filter := bson.M{}

	// If not all documents are requested, apply additional filter conditions
	categoryName := c.DefaultQuery("category", "")
	if categoryName != "" {
		// Lookup the category by slug
		var category entities.Category
		err := categoryCollection.FindOne(context.Background(), bson.M{"slug": categoryName}).Decode(&category)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Category not found"})
			return
		}

		categoryIDs, err := searchChildrenIDs(*category.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		}

		// fmt.Println(categoryIDs)
		if categoryIDs != nil {
			var categoryID []string
			for _, catID := range categoryIDs {

				categoryID = append(categoryID, catID.ID.Hex())
			}

			filter["categoryId"] = bson.M{"$in": categoryID}

		} else {
			filter["categoryId"] = category.ID.Hex()
		}

	}

	// Pagination parameters from the query
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calculate skip value for pagination
	skip := (page - 1) * limit

	// Calculate total number of documents in the collection
	totalDocs, err := proCollection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
		return
	}

	results, err := proCollection.Find(ctx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error finding products"})
		return
	}
	defer results.Close(ctx)

	var products []entities.Products
	var productsWithQuantity, productsWithoutQuantity []entities.Products

	for results.Next(ctx) {
		var pro entities.Products
		err := results.Decode(&pro)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		hasQuantity := false
		for _, p := range pro.Variations {
			if p.Quantity != 0 && len(pro.Variations) != 0 {
				hasQuantity = true
				break
			}
		}

		if hasQuantity {
			productsWithQuantity = append(productsWithQuantity, pro)
		} else {
			productsWithoutQuantity = append(productsWithoutQuantity, pro)
		}
	}
	products = append(products, productsWithQuantity...)
	products = append(products, productsWithoutQuantity...)

	// Calculate total number of pages based on the limit
	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

	// Determine if there are previous and next pages
	hasPrevPage := page > 1
	hasNextPage := page < totalPages

	// Prepare the custom response with selected fields
	var customProducts []gin.H
	for _, product := range products {
		customProduct := gin.H{
			"_id":             product.ID,
			"notExist":        product.NotExist,
			"amazing":         product.Amazing,
			"productType":     product.ProductType,
			"images":          product.Images,
			"name":            product.Name,
			"price":           product.Price,
			"discountPercent": product.DiscountPercent,
			"stock":           product.Stock,
			"slug":            product.Slug,
			"variations":      product.Variations,
			"salesNumber":     product.SalesNumber,
			"bannerUrl":       product.BannerUrl,
		}
		customProducts = append(customProducts, customProduct)
	}

	// Prepare the response with custom products and pagination information
	response := gin.H{
		"docs":          customProducts,
		"totalDocs":     totalDocs,
		"limit":         limit,
		"totalPages":    totalPages,
		"page":          page,
		"pagingCounter": skip + 1,
		"hasPrevPage":   hasPrevPage,
		"hasNextPage":   hasNextPage,
	}

	// Set prevPage and nextPage values based on the current page
	if hasPrevPage {
		response["prevPage"] = page - 1
	} else {
		response["prevPage"] = nil
	}

	if hasNextPage {
		response["nextPage"] = page + 1
	} else {
		response["nextPage"] = nil
	}

	c.JSON(http.StatusOK, response)
}

// ********************************************************************************
func searchChildrenIDs(categoryID primitive.ObjectID) ([]entities.Category, error) {
	cur, err := categoryCollection.Find(context.Background(), bson.M{"parent": categoryID})

	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())

	var categoryIDs []entities.Category
	for cur.Next(context.Background()) {
		var category entities.Category
		err := cur.Decode(&category)
		if err != nil {
			return nil, err
		}

		// Recursively search for children IDs
		childrenIDs, err := searchChildrenIDs(*category.ID)
		if err != nil {
			return nil, err
		}

		// Append the current category and its children
		categoryIDs = append(categoryIDs, category)
		categoryIDs = append(categoryIDs, childrenIDs...)
	}

	return categoryIDs, nil
}

//************************************************************************************
func UndefindProduct(c *gin.Context) {
	// products,err:=GetSearch(c)
	// var p entities.Products
	// cur, err := proCollection.Find(c, bson.M{})
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
	// 	return
	// }

	// var products []entities.Products

	// defer cur.Close(c)
	// for cur.Next(c) {
	// 	var pro entities.Products
	// 	err := cur.Decode(&pro)
	// 	if err != nil {
	// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
	// 		return
	// 	}
	// 	// fmt.Println(pro.ID)
	// 	products = append(products, pro)
	// }
	// // for _, p := range products {
	// // 	// fmt.Println(p.ID)
	// // }

	// // c.JSON(http.StatusOK, gin.H{"pages": 1, "docs": products})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "category", "body": nil})
}

// ProRouter provides ...
// @Summary Get mix products
// @Description Get mix products
// @Tags products
// @Accept json
// @Produce json
// @Success 200 {object} response.MixProductsResponse
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /api/mix-products [get]
func GetMixProducts(c *gin.Context) {
	var mixProducts []entities.MixProducts
	cur, err := mixProductCollection.Find(c, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"massage": "Not Find Collection"})
		return
	}
	defer cur.Close(c)
	for cur.Next(c) {
		var mix entities.MixProducts
		err := cur.Decode(&mix)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return

		}
		mixProducts = append(mixProducts, mix)

	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "mix_products", "body": mixProducts})

}

// func PostMixesProduct(c *gin.Context) {
// 	var mix entities.Mixes

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

// 	userId := claims.Id
// 	username := claims.Username
// 	if err := c.ShouldBindJSON(&mix); err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}
// 	mix.ID = primitive.NewObjectID()
// 	mix.UserId = userId
// 	mix.CreatedAt = time.Now()
// 	mix.UpdatedAt = time.Now()
// 	_, err := mixesCollection.InsertOne(c, mix)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Check if a document with the username already exists in cartCollection
// 	filter := bson.M{"username": username}
// 	var existingDoc entities.Catrs
// 	err = cartCollection.FindOne(c, filter).Decode(&existingDoc)
// 	if err == nil {
// 		// If document exists, update it with the new mix
// 		update := bson.M{"$set": bson.M{"mix": mix.ID}}
// 		_, err := cartCollection.UpdateOne(c, filter, update)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
// 			return
// 		}
// 	} else {
// 		// If document does not exist, create a new one
// 		var cart entities.Catrs
// 		cart.Id = primitive.NewObjectID()
// 		cart.Status = "active"
// 		cart.UserName = username
// 		cart.Mix = mix.ID
// 		cart.CreatedAt = time.Now()
// 		cart.UpdatedAt = time.Now()

// 		// Initialize Products field if nil
// 		if cart.Products == nil {
// 			cart.Products = make([]entities.ComeProduct, 0)
// 		}

// 		// Insert the new document into the database
// 		_, err := cartCollection.InsertOne(c, cart)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
// 			return
// 		}
// 	}

// 	c.JSON(http.StatusOK, gin.H{"success": true, "message": "mix"})
// 	c.JSON(http.StatusNoContent, gin.H{})
// }

func PostMixesProduct(c *gin.Context) {
	var mixes []entities.Mixes

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

	userId := claims.Id
	username := claims.Username

	if err := c.ShouldBindJSON(&mixes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, mix := range mixes {
		mix.ID = primitive.NewObjectID()
		mix.UserId = userId
		mix.CreatedAt = time.Now()
		mix.UpdatedAt = time.Now()

		_, err := mixesCollection.InsertOne(c, mix)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Append the mix ID to the mixIDs slice
		mixIDs := []primitive.ObjectID{mix.ID}

		filter := bson.M{"username": username}
		var existingDoc entities.Catrs
		err = cartCollection.FindOne(c, filter).Decode(&existingDoc)
		if err == nil {
			// If document exists, update it with the new mix
			update := bson.M{"$push": bson.M{"mix": bson.M{"$each": mixIDs}}}
			_, err := cartCollection.UpdateOne(c, filter, update)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
				return
			}
		} else {
			// If document does not exist, create a new one
			var cart entities.Catrs
			cart.Id = primitive.NewObjectID()
			cart.Status = "active"
			cart.UserName = username
			cart.Mix = mixIDs
			cart.CreatedAt = time.Now()
			cart.UpdatedAt = time.Now()

			// Initialize Products field if nil
			if cart.Products == nil {
				cart.Products = make([]entities.ComeProduct, 0)
			}

			// Insert the new document into the database
			_, err := cartCollection.InsertOne(c, cart)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "mix"})
	c.JSON(http.StatusNoContent, gin.H{})
}

func DeleteMixofCart(c *gin.Context) {
	// Parse the 'id' parameter from the URL
	id := c.Param("id")

	// Convert the 'id' parameter to a primitive.ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'id' parameter"})
		return
	}

	// Get the token claims
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

	filter := bson.M{"username": username}
	var existingDoc entities.Catrs
	err = cartCollection.FindOne(c, filter).Decode(&existingDoc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	// Check if the mix ID is in the mix array
	var mixIndexToDelete = -1
	for i, mixID := range existingDoc.Mix {
		if mixID == objectID {
			mixIndexToDelete = i
			break
		}
	}

	// If mix ID is found in the mix array, remove it
	if mixIndexToDelete != -1 {
		existingDoc.Mix = append(existingDoc.Mix[:mixIndexToDelete], existingDoc.Mix[mixIndexToDelete+1:]...)

		// Update the existing document in the database
		update := bson.M{"$set": bson.M{
			"mix":       existingDoc.Mix,
			"updatedAt": time.Now(),
		}}
		_, err = cartCollection.UpdateOne(c, filter, update)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		// c.JSON(http.StatusCreated, gin.H{"message": "cart_edited", "success": true, "body": gin.H{}})
		// c.JSON(http.StatusNoContent, gin.H{})
		// return
	}

	// // Delete the mix from the cart collection
	// filter := bson.M{"username": claims.Username, "mix": objectID}
	// update := bson.M{"$unset": bson.M{"mix": ""}}
	// _, err = cartCollection.UpdateOne(c, filter, update)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete mix from cart"})
	// 	return
	// }

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "mix_deleted"})
}

func OptionsMixByID(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

//*********************************************************************

func AddProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := auth.CheckUserType(c, "admin"); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return

		}
		var pro entities.Products
		if err := c.ShouldBindJSON(&pro); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "product not truth"})
			return
		}

		pro.CreatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		pro.UpdatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		_, err := proCollection.InsertOne(c, &pro)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"message": pro})

	}
}
func GetAllCountProducts(c *gin.Context) {
	var products int64
	count, err := proCollection.CountDocuments(c, bson.M{})
	if err != nil {
		// Handle error
		return
	}
	products = count

	c.JSON(http.StatusOK, gin.H{"count": products})
}

// func GetProductsByField(c *gin.Context) {

// 	filter := bson.M{}

// 	// Check if the 'amazing' parameter is set to "true"
// 	if c.DefaultQuery("amazing", "") == "true" {
// 		// If 'amazing' is "true," set the filter to fetch amazing products
// 		filter["amazing"] = true

// 	}

// 	// onlyExistsParam := c.DefaultQuery("onlyexists", "")
// 	// isNewParam := c.DefaultQuery("new", "")

// 	// if onlyExistsParam == "true" {

// 	// 	filter = bson.M{}
// 	// }

// 	// if onlyExistsParam == "true" && isNewParam == "1" {

// 	// 	filter = bson.M{}
// 	// }

// 	categoryId := c.DefaultQuery("categoryid", "")
// 	if categoryId != "" {
// 		// Convert the categoryId string to ObjectID
// 		objectID, err := primitive.ObjectIDFromHex(categoryId)
// 		if err != nil {
// 			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid Object ID"})
// 			return
// 		}

// 		var category entities.Category
// 		err = categoryCollection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&category)
// 		if err != nil {
// 			c.JSON(http.StatusNotFound, gin.H{"message": "Category not found"})
// 			return
// 		}

// 		categoryIDs, err := searchChildrenIDs(*category.ID)
// 		if err != nil {
// 			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
// 		}

// 		// fmt.Println(categoryIDs)
// 		if categoryIDs != nil {
// 			var categoryID []string
// 			for _, catID := range categoryIDs {

// 				categoryID = append(categoryID, catID.ID.Hex())
// 			}

// 			filter["categoryId"] = bson.M{"$in": categoryID}

// 		} else {
// 			filter["categoryId"] = category.ID.Hex()
// 		}
// 	}

// 	categoryName := c.DefaultQuery("category", "")
// 	if categoryName != "" {
// 		// Lookup the category by slug
// 		var category entities.Category
// 		err := categoryCollection.FindOne(context.Background(), bson.M{"slug": categoryName}).Decode(&category)
// 		if err != nil {
// 			c.JSON(http.StatusNotFound, gin.H{"message": "Category not found"})
// 			return
// 		}

// 		categoryIDs, err := searchChildrenIDs(*category.ID)
// 		if err != nil {
// 			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
// 		}

// 		if categoryIDs != nil {
// 			var categoryID []string
// 			for _, catID := range categoryIDs {

// 				categoryID = append(categoryID, catID.ID.Hex())
// 			}

// 			filter["categoryId"] = bson.M{"$in": categoryID}

// 		} else {
// 			filter["categoryId"] = category.ID.Hex()
// 		}

// 	}

// 	// Pagination parameters from the query
// 	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
// 	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	// Calculate skip value for pagination
// 	skip := (page - 1) * limit

// 	// Calculate total number of documents in the collection
// 	totalDocs, err := proCollection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to count products"})
// 		return
// 	}
// 	// fmt.Println("totalDocts:", totalDocs)
// 	// Fetch products based on the constructed filter
// 	results, err := proCollection.Find(ctx, filter, options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)))
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query products"})
// 		return
// 	}

// 	defer results.Close(ctx)
// 	var products []entities.Products

// 	for results.Next(ctx) {
// 		var pro entities.Products
// 		err := results.Decode(&pro)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
// 			return
// 		}

// 		products = append(products, pro)
// 	}

// 	// Calculate total number of pages based on the limit
// 	totalPages := int(math.Ceil(float64(totalDocs) / float64(limit)))

// 	// Determine if there are previous and next pages
// 	hasPrevPage := page > 1
// 	hasNextPage := page < totalPages

// 	// Prepare the custom response with selected fields
// 	var customProducts []gin.H
// 	for _, product := range products {
// 		customProduct := gin.H{
// 			"_id":             product.ID,
// 			"notExist":        product.NotExist,
// 			"amazing":         product.Amazing,
// 			"productType":     product.ProductType,
// 			"images":          product.Images,
// 			"name":            product.Name,
// 			"price":           product.Price,
// 			"discountPercent": product.DiscountPercent,
// 			"stock":           product.Stock,
// 			"slug":            product.Slug,
// 			"variations":      product.Variations,
// 			"salesNumber":     product.SalesNumber,
// 			"bannerUrl":       product.BannerUrl,
// 		}
// 		customProducts = append(customProducts, customProduct)
// 	}

// 	// Prepare the response with custom products and pagination information
// 	response := gin.H{
// 		"docs":          customProducts,
// 		"totalDocs":     totalDocs,
// 		"limit":         limit,
// 		"totalPages":    totalPages,
// 		"page":          page,
// 		"pagingCounter": skip + 1,
// 		"hasPrevPage":   hasPrevPage,
// 		"hasNextPage":   hasNextPage,
// 	}

// 	// Set prevPage and nextPage values based on the current page
// 	if hasPrevPage {
// 		response["prevPage"] = page - 1
// 	} else {
// 		response["prevPage"] = nil
// 	}

// 	if hasNextPage {
// 		response["nextPage"] = page + 1
// 	} else {
// 		response["nextPage"] = nil
// 	}

// 	c.JSON(http.StatusOK, response)

// }
