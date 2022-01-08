package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fengqi/base66"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

var (
	help    bool
	listen  *string
	domain  *string
	storage = "data"
	lock    *sync.RWMutex
)

type apiJsonResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func init() {
	flag.BoolVar(&help, "h", false, "this help")
	listen = flag.String("l", ":8080", "listen")
	domain = flag.String("d", "short.example.com:8080", "domain")
	lock = new(sync.RWMutex)
}

func main() {
	flag.Parse()
	if help {
		flag.Usage()
		return
	}

	http.HandleFunc("/", handleAll)
	http.HandleFunc("/api", handleApi)
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { return })

	server := &http.Server{Addr: *listen, Handler: nil}

	go func() {
		log.Printf("http server run at: %s\n", *listen)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("http server run err: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	sig := <-quit
	log.Printf("recevce quit signal: %d\n", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("http server shutdown err: %v\n", err)
	}
}

func handleAll(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		panic(err)
	}

	short := r.URL.Path[1:]
	if short != "" {
		url := short2Url(short)
		if checkUrl(url) {
			http.Redirect(w, r, url, 302)
			return
		}
	}

	tp := template.Must(template.ParseFiles("./static/index.html"))
	_ = tp.Execute(w, nil)
}

func handleApi(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		panic(err)
	}

	response := apiJsonResponse{Code: 200, Msg: "Ok"}
	url := r.Form.Get("url")

	if checkUrl(url) {
		response.Data = url2Short(url)
	} else {
		response.Code = 500
		response.Msg = "url syntax error"
	}

	res, _ := json.Marshal(response)
	_, err = w.Write(res)
	if err != nil {
		panic(err)
	}
}

func url2Short(url string) string {
	head := fmt.Sprintf("%02s", base66.Encode(uint64(len(url))))
	file := fmt.Sprintf("./%s/%s.dat", storage, head[1:2])

	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		panic(err)
	}
	body := base66.Encode(uint64(stat.Size()))

	lock.Lock()
	defer lock.Unlock()
	_, err = f.WriteString(url)
	if err != nil {
		panic(err)
	}

	return head + body
}

func short2Url(short string) string {
	file := fmt.Sprintf("./%s/%s.dat", storage, short[1:2])
	f, err := os.OpenFile(file, os.O_RDONLY, 0)
	if err != nil {
		return ""
	}
	defer f.Close()

	length := base66.Decode(short[0:2])
	offset := base66.Decode(short[2:])

	_, err = f.Seek(int64(offset), 0)
	bytes := make([]byte, length)
	_, err = f.Read(bytes)
	if err != nil {
		return ""
	}

	return string(bytes)
}

func checkUrl(url string) (b bool) {
	if len(url) == 0 || len(url) > 2048 {
		return false
	}

	if strings.Index(url, *domain) > -1 || strings.Index(url, *listen) > -1 {
		return false
	}

	if url[0:7] == "http://" || url[0:8] == "https://" {
		return true
	}

	return false
}
