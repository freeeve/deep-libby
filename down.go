package main

import (
	"fmt"
	"net/http"
)

const port = "443"

func main() {

	rootServeMux := http.NewServeMux()
	uiServeMux := http.NewServeMux()
	uiServeMux.Handle("GET /", http.HandlerFunc(uiHandler))

	rootServeMux.Handle("/", uiServeMux)

	privKey := "/etc/letsencrypt/live/deeplibby.com/privkey.pem"
	certFile := "/etc/letsencrypt/live/deeplibby.com/fullchain.pem"
	err := http.ListenAndServeTLS(fmt.Sprintf("0.0.0.0:%s", port), certFile, privKey, rootServeMux)
	if err != nil {
		panic(err)
	}
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Add("Content-Type", "text/html")
	uiBody := []byte(`
<!DOCTYPE html>
<html>
<head>
<title>DeepLibby</title>
</head>
<body>
<h1>DeepLibby is down for maintenance and will be back soon.</h1>
</body>
</html>
`)
	_, err := w.Write(uiBody)
	if err != nil {
		panic(err)
	}
}
