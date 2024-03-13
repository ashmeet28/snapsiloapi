package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type CodesSentBox struct {
	mu        sync.Mutex
	codesSent map[string]struct {
		c string
		t int64
	}
}

func (box *CodesSentBox) addCode(e string, c string) {
	box.mu.Lock()
	defer box.mu.Unlock()

	var MAX_LIFETIME int64 = 600
	var CLEANUP_TRIGGER int = 10000

	if len(box.codesSent) > CLEANUP_TRIGGER {
		for e, cs := range box.codesSent {
			if time.Now().Unix() < cs.t+MAX_LIFETIME {
				delete(box.codesSent, e)
			}
		}
	}

	var codeSent struct {
		c string
		t int64
	}

	codeSent.c = c
	codeSent.t = time.Now().Unix()
	box.codesSent[e] = codeSent
}

func (box *CodesSentBox) isCodeValid(e string, c string) bool {
	box.mu.Lock()
	defer box.mu.Unlock()

	var MAX_LIFETIME int64 = 600

	cs, ok := box.codesSent[e]
	if ok && cs.c == c && time.Now().Unix() < cs.t+MAX_LIFETIME {
		delete(box.codesSent, e)
		return true
	}

	return false
}

func createEmailVerificationCode() string {
	var c string
	for len(c) < 6 {
		c += func() string {
			n, err := rand.Int(rand.Reader, big.NewInt(10))
			if err != nil {
				fmt.Println("Error while getting random number")
				os.Exit(1)
			}
			return strconv.FormatInt(n.Int64(), 10)
		}()
	}

	return c
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(os.Getenv("SSAPI_WEBSITE"))))

	var codesSentStorage CodesSentBox

	codesSentStorage.codesSent = make(map[string]struct {
		c string
		t int64
	})

	type ReqJSON10 struct {
		EmailAddress string `json:"email_address"`
	}

	http.HandleFunc("/api/send-code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var j ReqJSON10
		err := json.NewDecoder(r.Body).Decode(&j)

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		c := createEmailVerificationCode()

		codesSentStorage.addCode(j.EmailAddress, c)

		fmt.Println(j.EmailAddress + ":" + c)

		w.WriteHeader(http.StatusOK)
	})

	type ReqJSON11 struct {
		EmailAddress     string `json:"email_address"`
		VerificationCode string `json:"verification_code"`
	}

	type ResJSON11 struct {
		AuthToken string `json:"auth_token"`
	}

	http.HandleFunc("/api/verify-code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var j ReqJSON11
		json.NewDecoder(r.Body).Decode(&j)

		if codesSentStorage.isCodeValid(j.EmailAddress, j.VerificationCode) {
			b := make([]byte, 16)
			_, err := rand.Read(b)
			if err != nil {
				fmt.Println("Error while getting random bytes")
				os.Exit(1)
			}
			var j ResJSON11
			j.AuthToken = hex.EncodeToString(b)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(j)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	http.ListenAndServe(":8080", nil)
}
