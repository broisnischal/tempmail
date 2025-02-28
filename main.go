package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Email struct {
	From    string    `json:"from"`
	To      string    `json:"to"`
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
	SentAt  time.Time `json:"sent_at"`
}

type Backend struct {
	rdb *redis.Client
}

func NewBackend(rdb *redis.Client) *Backend {
	return &Backend{rdb: rdb}
}

func (b *Backend) NewSession(conn *smtp.Conn) (smtp.Session, error) {
	return &Session{backend: b}, nil
}

type Session struct {
	backend *Backend
	from    string
	to      []string
}

func (s *Session) AuthPlain(username, password string) error {
	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if strings.HasSuffix(to, "@snehaa.store") {
		s.to = append(s.to, to)
		return nil
	}
	return smtp.ErrAuthRequired
}

func (s *Session) Data(r io.Reader) error {
	body, _ := io.ReadAll(r)
	msg, err := mail.ReadMessage(bytes.NewReader(body))
	if err != nil {
		return err
	}

	subject := msg.Header.Get("Subject")
	bodyContent, _ := io.ReadAll(msg.Body)

	for _, recipient := range s.to {
		email := Email{
			From:    s.from,
			To:      recipient,
			Subject: subject,
			Body:    string(bodyContent),
			SentAt:  time.Now(),
		}

		// Store email in Redis
		emailJSON, _ := json.Marshal(email)
		ctx := context.Background()

		// Add to list of emails for recipient
		s.backend.rdb.LPush(ctx, "emails:"+recipient, emailJSON)

		// Add to sorted set for expiration
		s.backend.rdb.ZAdd(ctx, "email_timestamps", redis.Z{
			Score:  float64(email.SentAt.Unix()),
			Member: recipient + ":" + string(emailJSON),
		})
	}

	return nil
}

func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *Session) Logout() error {
	return nil
}

func startSMTPServer(rdb *redis.Client) {
	backend := NewBackend(rdb)
	s := smtp.NewServer(backend)
	s.Addr = ":25"
	s.AllowInsecureAuth = true
	s.Domain = "snehaa.store"

	println("SMTP server running on :25")
	if err := s.ListenAndServe(); err != nil {
		panic(err)
	}
}

func startCleanup(rdb *redis.Client) {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		ctx := context.Background()
		now := time.Now().Add(-24 * time.Hour).Unix()

		// Get all emails older than 24 hours
		emails, err := rdb.ZRangeByScore(ctx, "email_timestamps", &redis.ZRangeBy{
			Min: "0",
			Max: fmt.Sprintf("%d", now),
		}).Result()

		if err != nil {
			continue
		}

		// Remove old emails
		for _, email := range emails {
			parts := strings.SplitN(email, ":", 2)
			if len(parts) != 2 {
				continue
			}
			recipient := parts[0]

			// Remove from recipient's list
			rdb.LRem(ctx, "emails:"+recipient, 0, parts[1])
			// Remove from sorted set
			rdb.ZRem(ctx, "email_timestamps", email)
		}
	}
}

func main() {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "redis"), getEnv("REDIS_PORT", "6379")),
		Password: "",
		DB:       0,
	})

	// Start cleanup job
	go startCleanup(rdb)

	// Start servers
	go api(rdb)

	startSMTPServer(rdb)
}

func api(rdb *redis.Client) {
	router := gin.Default()

	router.GET("/generate", func(c *gin.Context) {
		randomPart := uuid.New().String()[:8]
		email := fmt.Sprintf("%s@snehaa.store", randomPart)
		c.JSON(200, gin.H{"email": email})
	})

	router.GET("/inbox/:email", func(c *gin.Context) {
		email := strings.ToLower(c.Param("email"))
		ctx := context.Background()

		// Get all emails for recipient
		emailsJSON, err := rdb.LRange(ctx, "emails:"+email, 0, -1).Result()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to retrieve emails"})
			return
		}

		emails := make([]Email, 0, len(emailsJSON))
		for _, ej := range emailsJSON {
			var email Email
			if err := json.Unmarshal([]byte(ej), &email); err == nil {
				emails = append(emails, email)
			}
		}

		c.JSON(200, emails)
	})

	router.Run(":8989")
	println("HTTP API running on :8989")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// func startMailhog() {
// 	mailhog := mailhog.NewMailhog("localhost", "1025")
// 	mailhog.Start()
// }
