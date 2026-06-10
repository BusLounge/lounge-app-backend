package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID           uuid.UUID `json:"user_id"`
	Phone            string    `json:"phone"`
	Roles            []string  `json:"roles"`
	ProfileCompleted bool      `json:"profile_completed"`
	TokenType        string    `json:"token_type"`
	jwt.RegisteredClaims
}

func main() {
	userID := uuid.MustParse("4d54f70d-b75c-46fd-9f7a-82423e8a0c60")
	now := time.Now()
	claims := Claims{
		UserID:           userID,
		Phone:            "94771234567",
		Roles:            []string{"lounge_owner"},
		ProfileCompleted: true,
		TokenType:        "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "smarttransit-sms-auth",
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("your-256-bit-secret-key-here"))
	if err != nil {
		log.Fatalf("Error signing token: %v", err)
	}

	payloads := []struct {
		name string
		body string
	}{
		{
			name: "Basic package (only required fields, others omitted)",
			body: `{
				"package_name": "Basic Test Package",
				"package_type": "standard",
				"description": "Just basic access",
				"price": "500.00"
			}`,
		},
		{
			name: "Package with transport_status false, transport_mode null",
			body: `{
				"package_name": "No Transport Test Package",
				"package_type": "standard",
				"description": "No transport included",
				"price": "600.00",
				"transport_status": false
			}`,
		},
		{
			name: "Package with meal_status false, meal types null/omitted",
			body: `{
				"package_name": "No Meal Test Package",
				"package_type": "standard",
				"description": "No meals included",
				"price": "700.00",
				"meal_status": false
			}`,
		},
		{
			name: "Package with transport_status true but transport_mode missing",
			body: `{
				"package_name": "Missing Transport Mode Test Package",
				"package_type": "standard",
				"description": "Transport true but mode missing",
				"price": "800.00",
				"transport_status": true
			}`,
		},
	}

	client := &http.Client{}
	url := "http://localhost:8080/api/v1/lounges/e528941b-3c29-4b4e-87e8-6c2ae4ea8c34/special-packages"

	for _, p := range payloads {
		fmt.Printf("Testing payload: %s\n", p.name)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(p.body)))
		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenString)

		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error executing request: %v", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		fmt.Printf("Response Body: %s\n\n", string(respBody))
	}
}
