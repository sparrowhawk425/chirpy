package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/sparrowhawk425/chirpy/internal/auth"
	"github.com/sparrowhawk425/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
	tokenSecret    string
	polkaKey       string
}

func main() {
	// Load env data
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("Error accessing database: %v", err)
		return
	}
	apiCfg := apiConfig{
		dbQueries:   database.New(db),
		platform:    os.Getenv("PLATFORM"),
		tokenSecret: os.Getenv("TOKEN_SECRET"),
		polkaKey:    os.Getenv("POLKA_KEY"),
	}

	// Add server endpoints
	multiplexer := http.NewServeMux()

	// File server
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	multiplexer.Handle("/app/", apiCfg.middlewareMetricsInc(fileServerHandler))

	// Admin endpoints
	multiplexer.HandleFunc("GET /admin/metrics", apiCfg.metricsEndpoint)
	multiplexer.HandleFunc("POST /admin/reset", apiCfg.resetMetricsEndpoint)

	// API endpoints
	multiplexer.HandleFunc("GET /api/healthz", readinessEndpoint)
	multiplexer.HandleFunc("POST /api/refresh", apiCfg.refreshTokenEndpoint)
	multiplexer.HandleFunc("POST /api/revoke", apiCfg.revokeTokenEndpoint)
	multiplexer.HandleFunc("POST /api/users", apiCfg.createUserEndpoint)
	multiplexer.Handle("PUT /api/users", apiCfg.middlewareAuthenticateUser(apiCfg.updateUserEndpoint))
	multiplexer.HandleFunc("POST /api/login", apiCfg.loginEndpoint)
	multiplexer.Handle("POST /api/chirps", apiCfg.middlewareAuthenticateUser(apiCfg.createChirpEndpoint))
	multiplexer.HandleFunc("GET /api/chirps", apiCfg.getChirpsEndpoint)
	multiplexer.HandleFunc("GET /api/chirps/{chirpId}", apiCfg.getChirpEndpoint)
	multiplexer.Handle("DELETE /api/chirps/{chirpId}", apiCfg.middlewareAuthenticateUser(apiCfg.deleteChirpEndpoint))
	multiplexer.HandleFunc("POST /api/polka/webhooks", apiCfg.polkaWebhookEndpoint)

	// Start server
	server := http.Server{
		Handler: multiplexer,
		Addr:    ":8080",
	}
	server.ListenAndServe()
}

// Endpoint Handlers

func readinessEndpoint(w http.ResponseWriter, req *http.Request) {
	if req == nil {
		fmt.Println("HTTP request is not set")
		return
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) metricsEndpoint(w http.ResponseWriter, req *http.Request) {
	if req == nil {
		fmt.Println("HTTP request is not set")
		return
	}
	req.Header.Set("Content-Type", "text/html")
	w.WriteHeader(200)
	metricsContent :=
		`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
	fmt.Fprintf(w, metricsContent, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) resetMetricsEndpoint(w http.ResponseWriter, req *http.Request) {
	if req == nil {
		fmt.Println("HTTP request is not set")
		return
	}
	// Only development can execute this endpoint
	if cfg.platform != "dev" {
		w.WriteHeader(403)
		return
	}
	cfg.fileserverHits.Swap(0)
	if err := cfg.dbQueries.DeleteUsers(req.Context()); err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error resetting users table: %v", err))
		return
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, "Hits reset: %d", cfg.fileserverHits.Load())
}

type UserRequest struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}
type ErrorResponse struct {
	Error string `json:"error"`
}
type UserResponse struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

func (cfg *apiConfig) refreshTokenEndpoint(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	refreshToken, err := cfg.dbQueries.GetRefreshToken(req.Context(), token)
	if err != nil {
		sendErrorResponse(w, 401, fmt.Sprintf("Error getting refresh token: %v", err))
		return
	}
	if refreshToken.ExpiresAt.Before(time.Now()) || (refreshToken.RevokedAt.Valid && refreshToken.RevokedAt.Time.Before(time.Now())) {
		sendErrorResponse(w, 401, "Refresh token is not valid")
		return
	}
	user, err := cfg.dbQueries.GetUserFromRefreshToken(req.Context(), token)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error getting user from refresh token: %v", err))
		return
	}
	accessToken, err := auth.MakeJWT(user.ID, cfg.tokenSecret, time.Hour)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error creating JWT access token: %v", err))
		return
	}
	type Token struct {
		Token string `json:"token"`
	}
	sendJsonResponse(w, 200, Token{Token: accessToken})
}

func (cfg *apiConfig) revokeTokenEndpoint(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error getting bearer token: %v", err))
		return
	}
	refreshToken, err := cfg.dbQueries.GetRefreshToken(req.Context(), token)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error getting refresh token: %v", err))
		return
	}
	err = cfg.dbQueries.SetRevokedAtForRefreshToken(req.Context(), refreshToken.Token)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error revoking refresh token: %v", err))
		return
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) createUserEndpoint(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	userReq := UserRequest{}
	err := decoder.Decode(&userReq)
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Error decoding request data: %v", err))
		return
	}
	pwHash, err := auth.HashPassword(userReq.Password)
	if err != nil {
		sendErrorResponse(w, 500, "Error hashing user password")
		return
	}
	params := database.CreateUserParams{
		Email:          userReq.Email,
		HashedPassword: pwHash,
	}
	user, err := cfg.dbQueries.CreateUser(req.Context(), params)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Unable to create user: %v", err))
		return
	}
	userRes := UserResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}
	sendJsonResponse(w, 201, userRes)
}

func (cfg *apiConfig) loginEndpoint(w http.ResponseWriter, req *http.Request) {
	userReq := UserRequest{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&userReq)
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Error decoding request data: %v", err))
		return
	}
	user, err := cfg.dbQueries.GetUserByEmail(req.Context(), userReq.Email)
	if err != nil {
		sendErrorResponse(w, 404, fmt.Sprintf("Unable to find user with email %s", userReq.Email))
		return
	}
	ok, err := auth.CheckPasswordHash(userReq.Password, user.HashedPassword)
	if err != nil {
		sendErrorResponse(w, 500, "Error comparing password hash")
		return
	}
	if !ok {
		w.WriteHeader(401)
		return
	}
	expireIn := time.Hour
	token, err := auth.MakeJWT(user.ID, cfg.tokenSecret, expireIn)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error creating Auth token: %v", err))
		return
	}
	refreshToken := auth.MakeRefreshToken()
	params := database.CreateRefreshTokenParams{
		Token:  refreshToken,
		UserID: user.ID,
	}
	_, err = cfg.dbQueries.CreateRefreshToken(req.Context(), params)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error creating refresh token: %v", err))
		return
	}
	userRes := UserResponse{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		IsChirpyRed:  user.IsChirpyRed,
		Token:        token,
		RefreshToken: refreshToken,
	}
	sendJsonResponse(w, 200, userRes)
}

func (cfg *apiConfig) updateUserEndpoint(w http.ResponseWriter, req *http.Request, userId uuid.UUID) {

	// Parse request
	userReq := UserRequest{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&userReq)
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Error decoding request data: %v", err))
		return
	}
	hashedPw, err := auth.HashPassword(userReq.Password)
	if err != nil {
		sendErrorResponse(w, 500, "Error hashing updated password")
		return
	}
	params := database.UpdateUserParams{
		ID:             userId,
		Email:          userReq.Email,
		HashedPassword: hashedPw,
	}
	user, err := cfg.dbQueries.UpdateUser(req.Context(), params)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error updating user data: %v", err))
		return
	}
	userRes := UserResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}
	sendJsonResponse(w, 200, userRes)
}

type ChirpResponse struct {
	ID         uuid.UUID `json:"id"`
	Created_at time.Time `json:"created_at"`
	Updated_at time.Time `json:"updated_at"`
	Body       string    `json:"body"`
	UserId     uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) createChirpEndpoint(w http.ResponseWriter, req *http.Request, userId uuid.UUID) {

	// Parse Chirp from request
	type Chirp struct {
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(req.Body)
	chirp := Chirp{}
	err := decoder.Decode(&chirp)
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Error decoding request data: %v", err))
		return
	}

	// Validate chirp
	if len(chirp.Body) > 140 {
		sendErrorResponse(w, 400, "Chirp is too long")
		return
	} else {
		cleanBody := clean(chirp.Body)
		params := database.CreateChirpParams{
			Body:   cleanBody,
			UserID: userId,
		}
		chirpRes, err := cfg.dbQueries.CreateChirp(req.Context(), params)
		if err != nil {
			sendErrorResponse(w, 500, fmt.Sprintf("Unable to create chirp: %v", err))
			return
		}
		sendJsonResponse(w, 201, ChirpResponse{
			ID:         chirpRes.ID,
			Created_at: chirpRes.CreatedAt,
			Updated_at: chirpRes.UpdatedAt,
			Body:       chirpRes.Body,
			UserId:     chirpRes.UserID,
		})
	}
}

func (cfg *apiConfig) getChirpsEndpoint(w http.ResponseWriter, req *http.Request) {
	authorId := req.URL.Query().Get("author_id")
	var chirps []database.Chirp
	if authorId != "" {
		userId, err := uuid.Parse(authorId)
		if err != nil {
			sendErrorResponse(w, 400, fmt.Sprintf("Error parsing author ID: %v", err))
		}
		chirps, err = cfg.dbQueries.GetChirpsForUser(req.Context(), userId)
	} else {
		var err error
		chirps, err = cfg.dbQueries.GetChirps(req.Context())
		if err != nil {
			sendErrorResponse(w, 500, fmt.Sprintf("Error getting chirps: %v", err))
			return
		}
	}
	// Check if we want to revese sort order
	sortParam := req.URL.Query().Get("sort")
	if sortParam == "desc" {
		slices.SortFunc(chirps, func(a, b database.Chirp) int {
			if a.CreatedAt.Equal(b.CreatedAt) {
				return 0
			} else if a.CreatedAt.After(b.CreatedAt) {
				return -1
			}
			return 1
		})
	}
	chirpsResponse := serializeChirps(chirps)
	sendJsonResponse(w, 200, chirpsResponse)
}

func (cfg *apiConfig) getChirpEndpoint(w http.ResponseWriter, req *http.Request) {
	id, err := uuid.Parse(req.PathValue("chirpId"))
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Could not parse ID: %v", err))
		return
	}
	chirp, err := cfg.dbQueries.GetChirp(req.Context(), id)
	if err != nil {
		sendErrorResponse(w, 404, fmt.Sprintf("Chirp not found: %v", err))
		return
	}
	sendJsonResponse(w, 200, ChirpResponse{
		ID:         chirp.ID,
		Created_at: chirp.CreatedAt,
		Updated_at: chirp.UpdatedAt,
		Body:       chirp.Body,
		UserId:     chirp.UserID,
	})
}

func (cfg *apiConfig) deleteChirpEndpoint(w http.ResponseWriter, req *http.Request, userId uuid.UUID) {
	id, err := uuid.Parse(req.PathValue("chirpId"))
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Could not parse ID: %v", err))
		return
	}
	chirp, err := cfg.dbQueries.GetChirp(req.Context(), id)
	if err != nil {
		sendErrorResponse(w, 404, fmt.Sprintf("Error getting chirp: %v", err))
		return
	}
	if chirp.UserID != userId {
		sendErrorResponse(w, 403, "User is not permitted to delete this chirp")
		return
	}
	err = cfg.dbQueries.DeleteChirp(req.Context(), id)
	if err != nil {
		sendErrorResponse(w, 500, fmt.Sprintf("Error deleting chirp: %v", err))
		return
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) polkaWebhookEndpoint(w http.ResponseWriter, req *http.Request) {

	apiKey, err := auth.GetAPIKey(req.Header)
	if err != nil || apiKey != cfg.polkaKey {
		sendErrorResponse(w, 401, "Invalid ApiKey")
		return
	}
	type PolkaWebhook struct {
		Event string `json:"event"`
		Data  struct {
			UserId string `json:"user_id"`
		} `json:"data"`
	}

	decoder := json.NewDecoder(req.Body)
	webhook := PolkaWebhook{}
	err = decoder.Decode(&webhook)
	if err != nil {
		sendErrorResponse(w, 400, fmt.Sprintf("Error parsing webhook request: %v", err))
		return
	}
	if webhook.Event == "user.upgraded" {
		userId, err := uuid.Parse(webhook.Data.UserId)
		if err != nil {
			sendErrorResponse(w, 400, fmt.Sprintf("Error parsing user ID: %v", err))
			return
		}
		err = cfg.dbQueries.SetUserIsChirpyRed(req.Context(), userId)
		if err != nil {
			sendErrorResponse(w, 404, fmt.Sprintf("User not found: %v", err))
			return
		}
	}
	w.WriteHeader(204)
}

// Utility Functions

func clean(body string) string {
	profanity := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Fields(body)
	for i := range words {
		for _, profane := range profanity {
			if strings.ToLower(words[i]) == profane {
				words[i] = "****"
			}
		}
	}
	return strings.Join(words, " ")
}

func serializeChirps(chirps []database.Chirp) []ChirpResponse {
	response := make([]ChirpResponse, len(chirps))
	for i, chirp := range chirps {
		response[i] = ChirpResponse{
			ID:         chirp.ID,
			Created_at: chirp.CreatedAt,
			Updated_at: chirp.UpdatedAt,
			Body:       chirp.Body,
			UserId:     chirp.UserID,
		}
	}
	return response
}

func sendErrorResponse(w http.ResponseWriter, code int, errStr string) {
	log.Println(errStr)
	w.WriteHeader(code)
	res := ErrorResponse{
		Error: errStr,
	}
	resBody, err := json.Marshal(res)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		return
	}
	w.Write(resBody)
}

func sendJsonResponse(w http.ResponseWriter, code int, payload any) {
	w.WriteHeader(code)
	resBody, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		return
	}
	w.Write(resBody)
}

// Middleware

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) middlewareAuthenticateUser(handlerFunc func(http.ResponseWriter, *http.Request, uuid.UUID)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			sendErrorResponse(w, 401, fmt.Sprintf("Error authenticating user: %v", err))
			return
		}
		userId, err := auth.ValidateJWT(token, cfg.tokenSecret)
		if err != nil {
			sendErrorResponse(w, 401, fmt.Sprintf("Error authenticating user: %v", err))
			return
		}
		handlerFunc(w, r, userId)
	})
}
