package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	Database        = "Instagram" //Database Name
	UserCollection  = "Accounts"  // User Collection
	PostsCollection = "Posts"     // Post Collection
)

type User struct {
	Id       primitive.ObjectID `json:"Id,omitempty" bson:"_id,omitempty"`
	Name     string             `json:"Name" bson:"Name"`
	Email    string             `json:"Email" bson:"Email"`
	Password string             `json:"Password" bson:"Password"`
}

type Post struct {
	Id               primitive.ObjectID  `json:"Id,omitempty" bson:"_id,omitempty"`
	Uid              primitive.ObjectID  `json:"UId,omitempty" bson:"Uid,omitempty"`
	Caption          string              `json:"Caption,omitempty"`
	Image_URL        string              `json:"Image_URL,omitempty"`
	Posted_Timestamp primitive.Timestamp `json:"Timestamp,omitempty"`
}

var client *mongo.Client

func reportError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"message":"` + err.Error() + `"}`))

}

func GetUser(response http.ResponseWriter, request *http.Request) {

	response.Header().Set("Content-Type", "application/json")

	collection := client.Database(Database).Collection(UserCollection)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	uid := getId(request.URL.Path)

	id, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		reportError(response, err)
	}
	var user User

	FindId := bson.M{"_id": id}
	err = collection.FindOne(ctx, FindId).Decode(&user)
	if err != nil {

		reportError(response, errors.New("Invalid!,User does not exist"))
		return
	}
	json.NewEncoder(response).Encode(user)

}
func PostUser(response http.ResponseWriter, request *http.Request) {

	response.Header().Set("Content-Type", "application/json")

	collection := client.Database(Database).Collection(UserCollection)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var user User

	err := json.NewDecoder(request.Body).Decode(&user)
	if err != nil {
		reportError(response, err)
		return
	}

	//if User already exists with email,throw error

	FindEmail := bson.M{"Email": user.Email}
	Cursor, err := collection.Find(ctx, FindEmail)
	if err != nil {
		reportError(response, err)
	}

	var filteredUser []bson.M
	if err = Cursor.All(ctx, &filteredUser); err != nil {
		reportError(response, err)
		return
	}
	defer Cursor.Close(ctx)

	if len(filteredUser) != 0 {
		var alreadyExistsError = errors.New("User Already Exists")
		reportError(response, alreadyExistsError)
		return
	}

	hash := sha256.New()
	hash.Write([]byte(user.Password))
	user.Password = hex.EncodeToString(hash.Sum(nil))
	result, _ := collection.InsertOne(ctx, user)
	json.NewEncoder(response).Encode(result)
}

func GetPost(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	PC := client.Database(Database).Collection(PostsCollection)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	pid := getId(request.URL.Path)
	id, err := primitive.ObjectIDFromHex(pid)
	if err != nil {
		reportError(response, err)
	}
	var post Post

	FindId := bson.M{"_id": id}
	err = PC.FindOne(ctx, FindId).Decode(&post)
	if err != nil {
		reportError(response, errors.New("Post Doesnt Exists"))
		return
	}
	json.NewEncoder(response).Encode(post)
}
func PostPost(response http.ResponseWriter, request *http.Request) {
	UC := client.Database(Database).Collection(UserCollection)
	PC := client.Database(Database).Collection(PostsCollection)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	var post Post
	err := json.NewDecoder(request.Body).Decode(&post)
	if err != nil {
		reportError(response, err)
		return
	}

	var user User
	FindId := bson.M{"_id": post.Uid}

	err = UC.FindOne(ctx, FindId).Decode(&user)
	if err != nil {

		reportError(response, errors.New("User Doesnt Exists"))
		return
	}

	post.Posted_Timestamp = primitive.Timestamp{T: uint32(time.Now().Unix())}
	result, err := PC.InsertOne(ctx, post)
	if err != nil {
		reportError(response, err)
		return
	}
	json.NewEncoder(response).Encode(result)
}

func GetUserPost(response http.ResponseWriter, request *http.Request) {

	request.ParseForm()
	var limit int64 = 10
	var skip int64 = 0

	if len(request.Form) > 0 {
		for k, v := range request.Form {
			if k == "skip" {
				skip, _ = strconv.ParseInt(v[0], 10, 64)
			}
			if k == "limit" {
				limit, _ = strconv.ParseInt(v[0], 10, 64)
			}
		}
	}

	multiOptions := options.Find().SetLimit(limit).SetSkip(skip)

	log.Println(request.URL)
	response.Header().Set("Content-Type", "application/json")
	collection := client.Database(Database).Collection(PostsCollection)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	if request.Method != "GET" {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	uid := getId(request.URL.Path)
	id, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		reportError(response, err)
		return
	}

	FindPost := bson.M{"Uid": id}
	Cursor, err := collection.Find(ctx, FindPost, multiOptions)
	if err != nil {
		reportError(response, err)
		return
	}
	defer Cursor.Close(ctx)
	var userPosts []bson.M
	if err = Cursor.All(ctx, &userPosts); err != nil {
		reportError(response, err)
		return
	}

	json.NewEncoder(response).Encode(userPosts)
}

func getId(url string) string {
	p := strings.Split(url, "/")
	return p[len(p)-1]
}

func main() {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017")
	var err error
	client, err = mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("Error :%v", err)
	}

	http.HandleFunc("/users/", GetUser)
	http.HandleFunc("/users", PostUser)
	http.HandleFunc("/posts/", GetPost)
	http.HandleFunc("/posts", PostPost)
	http.HandleFunc("/posts/users/", GetUserPost)

	http.ListenAndServe(":8080", nil)
}
