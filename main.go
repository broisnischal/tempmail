package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Email struct {
	From    string
	To      string
	Subject string
	Body    string
}

type Backend struct {
	emails map[string][]*Email
}

var backend = &Backend{emails: make(map[string][]*Email)}

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
	// Check if the domain is "snehaa.store"
	if strings.HasSuffix(to, "@snehaa.store") {
		s.to = append(s.to, to)
		return nil
	}
	return smtp.ErrAuthRequired
}

func (s *Session) Data(r io.Reader) error {
	body, _ := io.ReadAll(r)

	for _, recipient := range s.to {
		email := &Email{
			From:    s.from,
			To:      recipient,
			Subject: "Temp Subject",
			Body:    string(body),
		}
		s.backend.emails[recipient] = append(s.backend.emails[recipient], email)
	}

	println("Email sent to ", s.to)
	return nil
}

func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *Session) Logout() error {
	return nil
}

func startSMTPServer() {
	s := smtp.NewServer(backend)
	s.Addr = ":25"
	s.AllowInsecureAuth = true
	s.Domain = "snehaa.store"

	println("SMTP `snehaa.store` running on :25")
	if err := s.ListenAndServe(); err != nil {
		panic(err)
	}
}

func main() {
	go api()
	startSMTPServer()
}

func api() {

	router := gin.Default()

	// Generate a new temp email address
	router.GET("/generate", func(c *gin.Context) {
		randomPart := uuid.New().String()[:8]
		email := fmt.Sprintf("%s@snehaa.store", randomPart) // Use your domain
		c.JSON(200, gin.H{"email": email})
	})

	// Get emails for an address
	router.GET("/inbox/:email", func(c *gin.Context) {
		email := c.Param("email")
		// Fetch emails from the database (replace "backend.emails" with your DB)
		emails := backend.emails[email]
		c.JSON(200, emails)
	})
	// Add to main() in api.go:
	// router.LoadHTMLGlob("templates/*")
	// router.GET("/", func(c *gin.Context) {
	// 	c.HTML(200, "index.html", nil)
	// })

	router.Run(":8080") // HTTP API on port 8080
	println("HTTP API running on :8080")
}
