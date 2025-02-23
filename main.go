package main

import (
    "fmt"
    "net/http"
    "os"

    "my_module/api"
)

func main() {
    port := "3000"
    if envPort := os.Getenv("PORT"); envPort != "" {
        port = envPort
    }

    http.HandleFunc("/", api.Handler)

    fmt.Println("Server running on http://localhost:" + port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        fmt.Println("Error:", err)
    }
}
