package controllers

import (
	"context"
	"ecommerce/database"
	"ecommerce/models"
	generate "ecommerce/tokens"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var UserCollection *mongo.Collection = database.UserData(database.Client, "Users")
var ProductCollection *mongo.Collection = database.ProductData(database.Client, "Products")
var Validate = validator.New()

func HashPassword(password string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		log.Panic(err)
	}
	return string(bytes)
}

func VerifyPassword(userpassword string, givenpassword string) (bool, string) {
	err := bcrypt.CompareHashAndPassword([]byte(givenpassword), []byte(userpassword))
	valid := true
	msg := ""
	if err != nil {
		msg = fmt.Sprintf("Login Or Passowrd is Incorerct")
		valid = false
	}
	return valid, msg
}

/**********************************************************************************************/

//function to signup
//accept a post request
//POST Request
//http://localhost:8000/users/signnup
/*
   "fisrt_name":"joseph",
   "last_name":"hermis",
   "email":"something@gmail.com",
   "phone":"1156422222",
   "password":"hashed:)"

*/

func SignUp() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		var user models.User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		validationErr := Validate.Struct(user)
		if validationErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": validationErr})
			return
		}
		count, err := UserCollection.CountDocuments(ctx, bson.M{"email": user.Email})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists"})
		}
		password := HashPassword(*user.Password)
		user.Password = &password
		count, err = UserCollection.CountDocuments(ctx, bson.M{"phone": user.Phone})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Phone is already in use"})
			return
		}
		user.Created_At, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.Updated_At, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.ID = primitive.NewObjectID()
		user.User_ID = user.ID.Hex()
		token, refreshtoken, _ := generate.TokenGenerator(*user.Email, *user.First_Name, *user.Last_Name, user.User_ID)
		user.Token = &token
		user.Refresh_Token = &refreshtoken
		user.UserCart = make([]models.ProductUser, 0)
		user.Address_Details = make([]models.Address, 0)
		user.Order_Status = make([]models.Order, 0)
		_, inserterr := UserCollection.InsertOne(ctx, user)
		if inserterr != nil {
			msg := fmt.Sprintf("not created")
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}
		defer cancel()
		c.JSON(http.StatusCreated, "Successfully Signed Up!!")
	}
}

/*********************************************************************************************************************************************************/

//function to generate login and check the user to create necessary fields in the db mostly as empty array
// Accepts a POST
/*
"email":"lololol@sss.com"
"password":"coollcollcoll"

*/

func Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		var user models.User
		var founduser models.User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}
		err := UserCollection.FindOne(ctx, bson.M{"email": user.Email}).Decode(&founduser)
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login or password incorrect"})
			return
		}
		PasswordIsValid, msg := VerifyPassword(*user.Password, *founduser.Password)
		defer cancel()
		if PasswordIsValid != true {
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			fmt.Println(msg)
			return
		}
		token, refreshToken, _ := generate.TokenGenerator(*founduser.Email, *founduser.First_Name, *founduser.Last_Name, founduser.User_ID)
		defer cancel()
		generate.UpdateAllTokens(token, refreshToken, founduser.User_ID)
		c.JSON(http.StatusFound, founduser)

	}
}

/*******************************************************************************************************/

//This is function to add products
//this is an admin part
//json should look like this
// post request : http://localhost:8080/admin/addproduct
/*
json

{
"product_name" : "pencil"
"price"        : 98
"rating"       : 10
"image"        : "image-url"
}




*/
func ProductViewerAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		var products models.Product
		defer cancel()
		if err := c.BindJSON(&products); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		products.Product_ID = primitive.NewObjectID()
		_, anyerr := ProductCollection.InsertOne(ctx, products)
		if anyerr != nil {
			msg := fmt.Sprintf("Not Created")
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}
		defer cancel()
		c.JSON(http.StatusOK, "Successfully added our Product Admin!!")
	}
}

// The Function to list all the productsin the database
//paging will be added and fixed soon

func SearchProduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		var productlist []models.Product
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		cursor, err := ProductCollection.Find(ctx, bson.D{{}})
		if err != nil {
			c.IndentedJSON(http.StatusInternalServerError, "Someting Went Wrong Please Try After Some Time")
			return
		}
		cursor.All(ctx, &productlist)
		defer cursor.Close(ctx)
		if err := cursor.Err(); err != nil {
			c.IndentedJSON(400, "invalid")
			return
		}
		defer cancel()
		c.IndentedJSON(200, productlist)

	}
}

// This is the function to search products based on alphabet name
func SearchProductByQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		var searchproducts []models.Product
		queryParam := c.Query("name")
		if queryParam == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Invalid Search Index"})
			c.Abort()
			return
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		searchquerydb, err := ProductCollection.Find(ctx, bson.M{"product_name": bson.M{"$regex": queryParam}})
		if err != nil {
			c.IndentedJSON(404, "something went wrong in fetching the dbquery")
			return
		}
		searchquerydb.All(ctx, &searchproducts)
		defer searchquerydb.Close(ctx)
		if err := searchquerydb.Err(); err != nil {
			c.IndentedJSON(400, "invalid request")
			return
		}
		defer cancel()
		c.IndentedJSON(200, searchproducts)
	}
}

/**************************************************CART********************************************************************************************************/

//function to add products to cart
// GET request
//http://localhost:8000/addtocart?id=xxxproduct_id&normal=xxxxxxuser_idxxxxxx

func AddToCart() gin.HandlerFunc {
	return func(c *gin.Context) {
		var productcart []models.ProductUser
		productqueryid := c.Query("id")
		userid := c.Query("normal")
		productid, _ := primitive.ObjectIDFromHex(productqueryid)
		if productqueryid == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Product Query not matched"})
			c.Abort()
			return
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		searchfromdb, err := ProductCollection.Find(ctx, bson.M{"_id": productid})
		if err != nil {
			c.IndentedJSON(http.StatusNotFound, "Invalid ID refer")
			return
		}
		searchfromdb.All(ctx, &productcart)
		defer cancel()
		id, err := primitive.ObjectIDFromHex(userid)
		if err != nil {
			fmt.Println(err)
		}
		filter := bson.D{primitive.E{Key: "_id", Value: id}}
		update := bson.D{{Key: "$push", Value: bson.D{primitive.E{Key: "usercart", Value: bson.D{{Key: "$each", Value: productcart}}}}}}
		_, err = UserCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(http.StatusInternalServerError, err)
		}
		c.IndentedJSON(200, "Successfully Added to the cart")
	}
}

/************************************************************************************************************************************/

//function to remove item from cart
//GET Request
//http://localhost:8000/addtocart?id=xxxproduct_id&normal=xxxxxxuser_idxxxxxx
func RemoveItem() gin.HandlerFunc {
	return func(c *gin.Context) {
		remove_id := c.Query("id")
		user_id := c.Query("normal")
		if remove_id == "" || user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Invalid Query"})
			c.Abort()
			return
		}
		removed_id, _ := primitive.ObjectIDFromHex(remove_id)
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			fmt.Println(err)
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.M{"$pull": bson.M{"usercart": bson.M{"_id": removed_id}}}
		_, err = UserCollection.UpdateMany(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(500, "Server Error")
			return
		}
		c.IndentedJSON(200, "Successfully removed from cart")
		defer cancel()
	}
}

/***************************************************************************************************************/

//function to get all items in the cart and total price
//GET request
//http://localhost:8000/listcart?id=xxxxxxuser_idxxxxxxxxxx
func GetItemFromCart() gin.HandlerFunc {
	return func(c *gin.Context) {
		var filledcart models.User
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"error": "invalid id"})
			c.Abort()
			return
		}
		usert_id, _ := primitive.ObjectIDFromHex(user_id)
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		err := UserCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: usert_id}}).Decode(&filledcart)
		if err != nil {
			c.IndentedJSON(500, "not id found")
			return
		}
		filter_match := bson.D{{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: usert_id}}}}
		unwind := bson.D{{Key: "$unwind", Value: bson.D{primitive.E{Key: "path", Value: "$usercart"}}}}
		grouping := bson.D{{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$_id"}, {Key: "total", Value: bson.D{primitive.E{Key: "$sum", Value: "$usercart.price"}}}}}}
		pointcursor, err := UserCollection.Aggregate(ctx, mongo.Pipeline{filter_match, unwind, grouping})
		if err != nil {
			fmt.Println(err)
		}
		var listing []bson.M
		if err = pointcursor.All(ctx, &listing); err != nil {
			panic(err)
		}
		for _, json := range listing {
			c.IndentedJSON(200, json["total"])
			c.IndentedJSON(200, filledcart.UserCart)
		}
		ctx.Done()
	}
}

/***********************************************************ADDRESS*************************************************************************/

//function to add the address and limited to 2
//home and work address
/*
{
"house_name":"jupyterlab",
"street_name":"notebook",
"city_name":"josua",
"pin_code":"685607"
}
The Post Request Url will look like this
POST
http://localhost:8000/addadress?id=user_id*************

*/

func AddAddress() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"error": "Invalid code"})
			c.Abort()
			return
		}
		address, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Error")
		}
		var addresses models.Address
		addresses.Address_id = primitive.NewObjectID()
		if err = c.BindJSON(&addresses); err != nil {
			c.IndentedJSON(http.StatusNotAcceptable, err.Error())
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		match_filter := bson.D{{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: address}}}}
		unwind := bson.D{{Key: "$unwind", Value: bson.D{primitive.E{Key: "path", Value: "$address"}}}}
		group := bson.D{{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$address_id"}, {Key: "count", Value: bson.D{primitive.E{Key: "$sum", Value: 1}}}}}}
		pointcursor, err := UserCollection.Aggregate(ctx, mongo.Pipeline{match_filter, unwind, group})
		if err != nil {
			c.IndentedJSON(500, "Internal Server Error")
		}
		var addressinfo []bson.M
		if err = pointcursor.All(ctx, &addressinfo); err != nil {
			panic(err)
		}
		var size int32
		for _, address_no := range addressinfo {
			count := address_no["count"]
			size = count.(int32)
		}
		if size < 2 {
			filter := bson.D{primitive.E{Key: "_id", Value: address}}
			update := bson.D{{Key: "$push", Value: bson.D{primitive.E{Key: "address", Value: addresses}}}}
			_, err := UserCollection.UpdateOne(ctx, filter, update)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			c.IndentedJSON(400, "Not Allowed ")
		}
		defer cancel()
		ctx.Done()
	}
}

/**********************************************************************************************************/

//function to edit the address put request
/*

{
"house_name":"jupyterlab",
"street_name":"notebook",
"city_name":"mars",
"pin_code":"12231997"
}
PUT
http://localhost:8000/edithomeaddress?id=xxxxxxxxxxxxxxxxxxxxxxxxx

*/

func EditHomeAddress() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Invalid"})
			c.Abort()
			return
		}
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, err)
		}
		var editaddress models.Address
		if err := c.BindJSON(&editaddress); err != nil {
			c.IndentedJSON(http.StatusBadRequest, err.Error())
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.D{{Key: "$set", Value: bson.D{primitive.E{Key: "address.0.house_name", Value: editaddress.House}, {Key: "address.0.street_name", Value: editaddress.Street}, {Key: "address.0.city_name", Value: editaddress.City}, {Key: "address.0.pin_code", Value: editaddress.Pincode}}}}
		_, err = UserCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(500, "Something Went Wrong")
			return
		}
		defer cancel()
		ctx.Done()
		c.IndentedJSON(200, "Successfully Updated the Home address")
	}
}

/*****************************************************************************************************/

//function to edit the work address put request
/*

{
"house_name":"jupyterlab",
"street_name":"notebook",
"city_name":"mars",
"pin_code":"12231997"
}
PUT
http://localhost:8000/editworkaddress?id=xxxxxxxxxxxxxxxxxxxxxxxxx

*/

func EditWorkAddress() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Wrong id not provided"})
			c.Abort()
			return
		}
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, err)
		}
		var editaddress models.Address
		if err := c.BindJSON(&editaddress); err != nil {
			c.IndentedJSON(http.StatusBadRequest, err.Error())
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.D{{Key: "$set", Value: bson.D{primitive.E{Key: "address.1.house_name", Value: editaddress.House}, {Key: "address.1.street_name", Value: editaddress.Street}, {Key: "address.1.city_name", Value: editaddress.City}, {Key: "address.1.pin_code", Value: editaddress.Pincode}}}}
		_, err = UserCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(500, "something Went wrong")
			return
		}
		defer cancel()
		ctx.Done()
		c.IndentedJSON(200, "Successfully updated the Work Address")
	}
}

/********************************************************************************************/

//function to delete the address here both the address will be removed fix soon
//GET request
//http://localhost:8000/deleteaddresses?id=xxxxxxxxxxxxxxxxxxxxxxxx

func DeleteAddress() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Invalid Search Index"})
			c.Abort()
			return
		}
		addresses := make([]models.Address, 0)
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Error")
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.D{{Key: "$set", Value: bson.D{primitive.E{Key: "address", Value: addresses}}}}
		_, err = UserCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(404, "Wromg")
			return
		}
		defer cancel()
		ctx.Done()
		c.IndentedJSON(200, "Successfully Deleted!")
	}
}

/***********************************************************************************************************************************************************************/

func BuyFromCart() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"error": "Invalid"})
			c.Abort()
			return
		}
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Error")
		}
		var getcartitems models.User
		var ordercart models.Order
		ordercart.Order_ID = primitive.NewObjectID()
		ordercart.Orderered_At = time.Now()
		ordercart.Order_Cart = make([]models.ProductUser, 0)
		ordercart.Payment_Method.COD = true
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		unwind := bson.D{{Key: "$unwind", Value: bson.D{primitive.E{Key: "path", Value: "$usercart"}}}}
		grouping := bson.D{{Key: "$group", Value: bson.D{primitive.E{Key: "_id", Value: "$_id"}, {Key: "total", Value: bson.D{primitive.E{Key: "$sum", Value: "$usercart.price"}}}}}}
		currentresults, err := UserCollection.Aggregate(ctx, mongo.Pipeline{unwind, grouping})

		ctx.Done()
		if err != nil {
			panic(err)
		}
		var getusercart []bson.M
		if err = currentresults.All(ctx, &getusercart); err != nil {
			panic(err)
		}
		var total_price int32
		for _, user_item := range getusercart {
			price := user_item["total"]
			total_price = price.(int32)
		}
		ordercart.Price = int(total_price)
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.D{{Key: "$push", Value: bson.D{primitive.E{Key: "orders", Value: ordercart}}}}
		UserCollection.UpdateMany(ctx, filter, update)
		err = UserCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: usert_id}}).Decode(&getcartitems)
		if err != nil {
			c.IndentedJSON(500, "something went wrong")
		}
		var ktx, kancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer kancel()
		filter2 := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update2 := bson.M{"$push": bson.M{"orders.$[].order_list": bson.M{"$each": getcartitems.UserCart}}}
		_, err = UserCollection.UpdateOne(ctx, filter2, update2)
		if err != nil {
			c.IndentedJSON(500, "something went wrong")
		}
		usercart_empty := make([]models.ProductUser, 0)
		filtered := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		updated := bson.D{{Key: "$set", Value: bson.D{primitive.E{Key: "usercart", Value: usercart_empty}}}}
		_, err = UserCollection.UpdateOne(ctx, filtered, updated)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Errror")
		}

		ktx.Done()
		c.IndentedJSON(200, "Successfully Placed the order")

	}
}

func InstantBuy() gin.HandlerFunc {
	return func(c *gin.Context) {
		item_id := c.Query("pid")
		user_id := c.Query("id")
		if item_id == "" || user_id == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusNotFound, gin.H{"Error": "Invalid Code"})
			c.Abort()
			return
		}
		itemt_id, err := primitive.ObjectIDFromHex(item_id)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Erroe")
		}
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, "Internal Server Error")
		}
		var product_details models.ProductUser
		var orders_detail models.Order
		orders_detail.Order_ID = primitive.NewObjectID()
		orders_detail.Orderered_At = time.Now()
		orders_detail.Order_Cart = make([]models.ProductUser, 0)
		orders_detail.Payment_Method.COD = true
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		err = ProductCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: itemt_id}}).Decode(&product_details)
		if err != nil {
			c.IndentedJSON(400, "Something Wrong happened")
		}
		orders_detail.Price = product_details.Price
		filter := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update := bson.D{{Key: "$push", Value: bson.D{primitive.E{Key: "orders", Value: orders_detail}}}}
		UserCollection.UpdateOne(ctx, filter, update)
		ctx.Done()
		filter2 := bson.D{primitive.E{Key: "_id", Value: usert_id}}
		update2 := bson.M{"$push": bson.M{"orders.$[].order_list": product_details}}
		_, err = UserCollection.UpdateOne(ctx, filter2, update2)
		if err != nil {
			c.IndentedJSON(400, "something wrong happened")
		}
		c.IndentedJSON(200, "Successully placed the order ")
		defer cancel()

	}
}

/*****BLACKLIST************
func Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		user_id := c.Query("id")
		if user_id == "" {
			c.Header("Content-Type", "application-json")
			c.JSON(http.StatusNoContent, gin.H{"Error": "Invalid"})
			c.Abort()
			return
		}
		usert_id, err := primitive.ObjectIDFromHex(user_id)
		if err != nil {
			c.IndentedJSON(500, "Something Went Wrong")
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		filter := bson.D{{"_id", usert_id}}
		update := bson.D{{"$unset", bson.D{{"token", ""}, {"refresh_token", ""}}}}
		_, err = UserCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			c.IndentedJSON(500, err)
		}

	}
}
//***************/
