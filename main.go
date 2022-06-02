package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type friend struct {
	FriendName  string `json:"FriendName"`
	Group       string `json:"Group"`
	DesiredFreq int    `json:"DesiredFreq"`
	LastContact string `json:"LastContact"`
}

var (
	envErr = godotenv.Load(".env")
	lock   = os.Getenv("LOCK")
)

func auth(r *http.Request) bool {
	queries := r.URL.Query()

	if key, exists := queries["key"]; exists {
		if key[0] == lock {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}

func getFriends(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("401 - Invalid Key"))
		return
	}

	// establish connection to database
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	results, err := db.Query("SELECT * FROM victor")
	if err != nil {
		panic(err.Error())
	}

	// create variable to store all friends
	friends := map[string][]friend{
		"friends": {},
	}

	for results.Next() {
		var friend friend
		err = results.Scan(&friend.FriendName, &friend.Group, &friend.DesiredFreq, &friend.LastContact)
		if err != nil {
			panic(err.Error())
		}

		friends["friends"] = append(friends["friends"], friend)
	}

	json.NewEncoder(w).Encode(friends)
}

func friendFunc(w http.ResponseWriter, r *http.Request) {
	if !auth(r) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("401 - Invalid Key"))
		return
	}

	params := mux.Vars(r)
	friendParam := params["friend"]

	// establish connection to database
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	if r.Method == "GET" {
		if friendExists(db, friendParam) {
			results, err := db.Query("SELECT * FROM victor WHERE friend_name = '" + friendParam + "' LIMIT 1")
			if err != nil {
				panic(err.Error())
			}

			// create variable to store all friends
			friends := map[string][]friend{
				"friends": {},
			}

			for results.Next() {
				var friend friend
				err = results.Scan(&friend.FriendName, &friend.Group, &friend.DesiredFreq, &friend.LastContact)
				if err != nil {
					panic(err.Error())
				}

				friends["friends"] = append(friends["friends"], friend)
			}

			json.NewEncoder(w).Encode(friends)
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 - Friend not found"))
		}
	}

	if r.Method == "DELETE" {
		// check if friend exists
		if friendExists(db, friendParam) {
			query := fmt.Sprintf("DELETE FROM victor WHERE friend_name='%s'", friendParam)

			_, err := db.Query(query)
			if err != nil {
				panic(err.Error())
			}

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("202 - Friend deleted: " + friendParam))
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 - Friend not found"))
		}
	}

	if r.Header.Get("Content-type") == "application/json" {

		if r.Method == "POST" {
			// read data received from client
			var newFriend friend
			reqBody, err := ioutil.ReadAll(r.Body)

			if err == nil {
				// convert JSON to object
				json.Unmarshal(reqBody, &newFriend)

				// check if all fields have been received
				if newFriend.FriendName == "" || newFriend.Group == "" || newFriend.DesiredFreq == 0 || newFriend.LastContact == "" {
					w.WriteHeader(http.StatusUnprocessableEntity)
					w.Write([]byte("422 - All fields must be filled out and in JSON format"))
					return
				}

				// check if friend exists
				if !friendExists(db, friendParam) {
					_, err := db.Query("INSERT INTO victor VALUES ('" + newFriend.FriendName + "', '" + newFriend.Group + "', " + strconv.Itoa(newFriend.DesiredFreq) + ", '" + newFriend.LastContact + "')")
					if err != nil {
						panic(err.Error())
					}

					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("201 - Friend added: " + friendParam))
				} else {
					w.WriteHeader(http.StatusConflict)
					w.Write([]byte("409 - Friend already exists"))
				}

			} else {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte("422 - Please supply information in JSON format"))
			}
		}

		if r.Method == "PUT" {
			// read data received from client
			var editFriend friend
			reqBody, err := ioutil.ReadAll(r.Body)

			if err == nil {
				// convert JSON to object
				json.Unmarshal(reqBody, &editFriend)

				// check if all fields have been received
				if editFriend.FriendName == "" || editFriend.Group == "" || editFriend.DesiredFreq == 0 || editFriend.LastContact == "" {
					w.WriteHeader(http.StatusUnprocessableEntity)
					w.Write([]byte("422 - All fields must be filled out and in JSON format"))
					return
				}

				// check if friend exists
				if friendExists(db, friendParam) {
					query := fmt.Sprintf("UPDATE victor SET `group`='%s', `desired_freq`=%d, `last_contact`='%s' WHERE `friend_name`='%s'", editFriend.Group, editFriend.DesiredFreq, editFriend.LastContact, editFriend.FriendName)
					_, err := db.Query(query)
					if err != nil {
						panic(err.Error())
					}

					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("201 - Friend details updated: " + friendParam))
				} else {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("404 - Friend not found"))
				}

			} else {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte("422 - Please supply information in JSON format"))
			}
		}

	}
}

func friendExists(db *sql.DB, friend string) (exists bool) {
	search, err := db.Query("SELECT EXISTS(SELECT * FROM victor WHERE friend_name = '" + friend + "')")
	if err != nil {
		panic(err.Error())
	}

	for search.Next() {
		err = search.Scan(&exists)
		if err != nil {
			panic(err.Error())
		}
	}

	return exists
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := mux.NewRouter()

	router.HandleFunc("/api/v1/friends", getFriends).Methods("GET")
	router.HandleFunc("/api/v1/friends/{friend}", friendFunc).Methods("GET", "PUT", "POST", "DELETE")

	log.Fatal(http.ListenAndServe(":"+port, router))
}
