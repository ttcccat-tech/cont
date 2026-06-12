package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    hash := "$2a$10$fX06nwaXuO.JMS8FDcomkuD6rPU2XIIHTZ71oa5zq2R/Ba0flSWzK"
    for _, pw := range []string{"cont1234", "admin", "password", "test1234", "cont12345"} {
        err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
        fmt.Printf("pw=%q: err=%v\n", pw, err)
    }
}
