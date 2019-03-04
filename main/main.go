package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		t := time.Now()
		genesisBlock := Block{}
		genesisBlock = Block{0, t.String(), 0, calculateHash(genesisBlock), "", difficulty, ""}
		spew.Dump(genesisBlock)

		mutex.Lock()
		BlockChain = append(BlockChain, genesisBlock)
		mutex.Unlock()
	}()
	log.Fatal(run())
}

// The number of 0's we want to lead the hash
const difficulty = 1

type Block struct {
	Index      int
	Timestamp  string
	BPM        int
	Hash       string
	PrevHash   string
	Difficulty int
	Nonce      string
}
var BlockChain []Block

type Message struct {
	BPM int
}

var mutex = &sync.Mutex{}

func run() error {
	mux := mux.NewRouter()
	mux.HandleFunc("/", handleGetBlockchain).Methods("GET")
	mux.HandleFunc("/", handleWriteBlock).Methods("POST")
	httpAddr := os.Getenv("ADDR")
	log.Println("Listening on ", httpAddr)
	s := &http.Server{
		Addr: ":" + httpAddr,
		Handler: mux,
		ReadTimeout: 10* time.Second,
		WriteTimeout: 10*time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err := s.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func handleWriteBlock(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	var m Message
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&m); err != nil {
		respondWithJSON(writer, request, http.StatusBadRequest, request.Body)
		return
	}
	defer request.Body.Close()

	//ensure atomicity when creating a new block
	mutex.Lock()
	newBlock := generateBlock(BlockChain[len(BlockChain)-1], m.BPM)
	mutex.Unlock()

	if isBlockValid(newBlock, BlockChain[len(BlockChain) -1], m.BPM) {
		BlockChain = append(BlockChain, newBlock)
		spew.Dump(BlockChain)
	}
	respondWithJSON(writer, request, http.StatusCreated, newBlock)
}

func isBlockValid(newBlock , block Block, i int) bool{
	if block.Index+1 != newBlock.Index || block.Hash != newBlock.PrevHash || calculateHash(newBlock) != newBlock.Hash{
		return false
	}
	return true
}

func isHashValid(hash string, difficulty int) bool{
	prefix := strings.Repeat("0", difficulty)
	return strings.HasPrefix(hash, prefix)
}

func calculateHash(block Block) string{
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.BPM) + block.PrevHash + block.Nonce
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, BPM int) Block {
	var newBlock Block

	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Difficulty = difficulty

	return mine(newBlock, 0)
}

func mine(block Block, i int) Block {
	hex := fmt.Sprintf("%x", i)
	block.Nonce = hex
	if !isHashValid(calculateHash(block), block.Difficulty){
		fmt.Printf(calculateHash(block), "calculating hash...")
		block = mine(block, i+1)
		time.Sleep(time.Second)
	} else {
		fmt.Printf(calculateHash(block), "<------------ hash value reached")
		block.Hash = calculateHash(block)
	}
	return block
}

func handleGetBlockchain(writer http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(BlockChain, "", " ")
	if err!= nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(writer, string(bytes))
}

func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {

	w.Header().Set("Content-Type", "application/json")
	response, err := json.MarshalIndent(payload, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
	}
	w.WriteHeader(code)
	w.Write(response)
}