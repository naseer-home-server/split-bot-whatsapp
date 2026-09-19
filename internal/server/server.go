package server

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/naseer2426/split-bot-whatsapp/internal/config"
	"github.com/naseer2426/split-bot-whatsapp/internal/whatsapp"
)

// Server represents the HTTP server
type Server struct {
	handler *whatsapp.Handler
	port    string
	router  *gin.Engine
}

// NewServer creates a new HTTP server instance
func NewServer(handler *whatsapp.Handler) *Server {
	port := config.Get().Server.Port

	router := gin.Default()

	// Allow CORS for all origins
	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:   []string{"Content-Length"},
	}))

	return &Server{
		handler: handler,
		port:    port,
		router:  router,
	}
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start() {
	s.setupRoutes()

	go func() {
		fmt.Printf("HTTP server starting on port %s\n", s.port)
		if err := s.router.Run(":" + s.port); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()
}

// setupRoutes registers all HTTP routes
func (s *Server) setupRoutes() {
	s.router.POST("/send_message_to_group", s.sendMessageToGroupHandler)
	s.router.POST("/send_message_to_user", s.sendMessageToUserHandler)
	s.router.POST("/send_message_to_chat", s.sendMessageToChatHandler)
	s.router.POST("/send_media_to_chat", s.sendMediaToChatHandler)
	s.router.POST("/typing", s.typingHandler)
	s.router.POST("/grafana_webhook", s.grafanaWebhookHandler)
	s.router.POST("/poll/create", s.pollCreateHandler)
	s.router.GET("/poll/status", s.pollStatusHandler)
	s.router.POST("/totals/create", s.totalsCreateHandler)
	s.router.GET("/totals/assignments", s.totalsAssignmentsGetHandler)
	s.router.PUT("/totals/assignments", s.totalsAssignmentsPutHandler)
}
