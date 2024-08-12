package main

import (
	"encoding/json"
	// "html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	port = ":8080"
)

var (
	// templates = template.Must(template.ParseFiles("terminal/login.html", "terminal/home.html", "terminal/signup.html", "terminal/admin.html", "terminal/create.html", "terminal/edit.html"))
	store = sessions.NewCookieStore([]byte("super-secret-key"))
	db    *gorm.DB
)

type User struct {
	ID       uint   `gorm:"primarykey;autoIncrement;"`
	Username string `gorm:"type:varchar(50);not null;unique"`
	Password string `gorm:"type:varchar(50);not null"`
	Email    string `gorm:"type:varchar(50);not null"`
	IsAdmin  bool   `gorm:"type:boolean;not null;default:false"`
}

func admin(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session_id")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if isAdmin, ok := session.Values["isAdmin"].(bool); !ok || !isAdmin {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
		return
	}

	var users []User
	result := db.Find(&users)
	if result.Error != nil {
		http.Error(w, "Error fetching users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func search(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	var users []User
	db.Where("username LIKE ? OR ID LIKE ?", "%"+search+"%", "%"+search+"%").Find(&users)

	session, _ := store.Get(r, "session_id")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func create(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		session, _ := store.Get(r, "session_id")
		if auth, ok := session.Values["isAdmin"].(bool); !ok || !auth {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user := User{Username: payload.Username, Password: payload.Password, Email: payload.Email}
		result := db.Create(&user)
		if result.Error != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Username already exists"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  " user created successfully",
			"redirect": "/admin"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func deleterec(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		idstr := mux.Vars(r)["id"]
		id, err := strconv.Atoi(idstr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		session, _ := store.Get(r, "session_id")
		if auth, ok := session.Values["isAdmin"].(bool); !ok || !auth {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		result := db.Delete(&User{}, id)
		if result.Error != nil {
			http.Error(w, "Error deleting user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  "User deleted successfully",
			"redirect": "/admin",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func editrec(w http.ResponseWriter, r *http.Request) {
	idstr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idstr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	session, _ := store.Get(r, "session_id")
	if auth, ok := session.Values["isAdmin"].(bool); !ok || !auth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost {
		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Email    string `json:"email"`
		}

		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		updatedUser := User{
			ID:       uint(id),
			Username: payload.Username,
			Password: payload.Password,
			Email:    payload.Email,
		}

		result := db.Model(&User{}).Where("id = ?", id).Updates(updatedUser)
		if result.Error != nil {
			http.Error(w, "Error updating user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  " user edited success",
			"redirect": "/admin"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func Strongpassword(password string) bool {
	Number := regexp.MustCompile(`[0-9]`).MatchString(password)
	Length := len(password) >= 7
	return Number && Length
}

func signup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var userDetails struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		err := json.NewDecoder(r.Body).Decode(&userDetails)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if !Strongpassword(userDetails.Password) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "Password must be at least 8 characters long and include numbers and symbols"})
			return
		}

		user := User{Username: userDetails.Username, Password: userDetails.Password, Email: userDetails.Email}
		result := db.Create(&user)
		if result.Error != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "Username already exists"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "User registered successfully!"})
		w.WriteHeader(http.StatusOK)
		return
	}

	http.ServeFile(w, r, "signup.html")
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var credentials struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		err := json.NewDecoder(r.Body).Decode(&credentials)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var user User
		result := db.Where("(username = ? or email = ?) AND password = ?", credentials.Username, credentials.Username, credentials.Password).First(&user)
		if result.Error != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "Invalid username or password"})
			return
		}

		session, _ := store.Get(r, "session_id")
		session.Values["authenticated"] = true
		session.Values["isAdmin"] = user.IsAdmin
		session.Save(r, w)

		response := map[string]string{
			"message": "Login successful!",
		}

		if user.IsAdmin {
			response["redirect"] = "/admin"
		} else {
			response["redirect"] = "/home"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	http.ServeFile(w, r, "login.html")
}

func home(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session_id")

    if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("Pragma", "no-cache")


    response := map[string]interface{}{
        "message": "Welcome to Home", 
    }

    json.NewEncoder(w).Encode(response)
}

func logout(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session_id")
    session.Values["authenticated"] = false
    session.Save(r, w)

    response := map[string]string{
        "message": "Logged out successfully",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func main() {
	var err error
	dsn := "root:razeem19@tcp(localhost:3306)/db3?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	log.Println("Connection successful")
	db.AutoMigrate(&User{})

	r := mux.NewRouter()
	r.HandleFunc("/", home)
	r.HandleFunc("/admin", admin).Methods(http.MethodGet)
	r.HandleFunc("/search", search).Methods(http.MethodGet)
	r.HandleFunc("/delete/{id:[0-9]+}", deleterec).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/edit/{id:[0-9]+}", editrec).Methods(http.MethodPost, http.MethodGet)
	r.HandleFunc("/home", home).Methods(http.MethodGet)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/logout", logout).Methods(http.MethodPost)
	r.HandleFunc("/signup", signup).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/create", create).Methods(http.MethodGet, http.MethodPost)

	log.Printf("Server started at http://localhost%s", port)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)
	log.Fatal(http.ListenAndServe(port, corsHandler(r)))
}
