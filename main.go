package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nfnt/resize"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

var jwtKey = []byte("your_access_secret")
var refreshKey = []byte("your_refresh_secret")

const accessTokenExpiry = 15 * time.Minute
const refreshTokenExpiry = 7 * 24 * time.Hour

type UploadSetting struct {
	Directory    string
	AllowedTypes []string
}

var uploadSettings = map[string]UploadSetting{
	"profile":  {"uploads/profile", []string{"image/jpeg", "image/png"}},
	"document": {"uploads/document", []string{"application/pdf"}},
}

func respondWithJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	respondWithJSON(w, map[string]string{"error": message})
}

func generateAccessToken(username, role string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(accessTokenExpiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func generateRefreshToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(refreshTokenExpiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(refreshKey)
}

func validateToken(tokenStr string, key []byte) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return key, nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}

// 로그인
func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	var passwordHash, role string
	err := db.QueryRow("SELECT password_hash, role FROM users WHERE username = ?", username).Scan(&passwordHash, &role)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	accessToken, _ := generateAccessToken(username, role)
	refreshToken, _ := generateRefreshToken(username)

	// Refresh Token 저장
	db.Exec("INSERT INTO refresh_tokens (username, token, expires_at) VALUES (?, ?, ?)", username, refreshToken, time.Now().Add(refreshTokenExpiry))

	setTokenCookies(w, accessToken, refreshToken)
	respondWithJSON(w, map[string]string{"message": "Login successful"})
}

// 토큰 쿠키 설정
func setTokenCookies(w http.ResponseWriter, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
		MaxAge:   int(accessTokenExpiry.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
		MaxAge:   int(refreshTokenExpiry.Seconds()),
	})
}

// 리프레시
func refreshHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Refresh token missing")
		return
	}

	claims, err := validateToken(cookie.Value, refreshKey)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	username := claims["username"].(string)

	var exists int
	db.QueryRow("SELECT COUNT(*) FROM refresh_tokens WHERE username = ? AND token = ?", username, cookie.Value).Scan(&exists)
	if exists == 0 {
		respondWithError(w, http.StatusUnauthorized, "Refresh token not found")
		return
	}

	var role string
	db.QueryRow("SELECT role FROM users WHERE username = ?", username).Scan(&role)

	newAccessToken, _ := generateAccessToken(username, role)
	newRefreshToken, _ := generateRefreshToken(username)

	// 이전 refresh_token 삭제
	db.Exec("DELETE FROM refresh_tokens WHERE username = ?", username)
	// 새 refresh_token 저장
	db.Exec("INSERT INTO refresh_tokens (username, token, expires_at) VALUES (?, ?, ?)", username, newRefreshToken, time.Now().Add(refreshTokenExpiry))

	setTokenCookies(w, newAccessToken, newRefreshToken)
	respondWithJSON(w, map[string]string{"message": "Token refreshed"})
}

// 인증 미들웨어
func authMiddleware(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Access token missing")
			return
		}

		claims, err := validateToken(cookie.Value, jwtKey)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Invalid access token")
			return
		}

		if requireAdmin && claims["role"] != "admin" {
			respondWithError(w, http.StatusForbidden, "Admin only")
			return
		}

		// 사용자 이름을 context로 넘길 수도 있음 (생략)
		next.ServeHTTP(w, r)
	}
}

// 파일 히스토리 기록
func logFileHistory(username, action string, fileID int64) {
	db.Exec("INSERT INTO file_history (file_id, username, action) VALUES (?, ?, ?)", fileID, username, action)
}

// 업로드 핸들러
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(20 << 20)

	useType := r.FormValue("useType")
	setting, ok := uploadSettings[useType]
	if !ok {
		respondWithError(w, http.StatusBadRequest, "Invalid useType")
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File missing")
		return
	}
	defer file.Close()

	contentType := handler.Header.Get("Content-Type")
	allowed := false
	for _, t := range setting.AllowedTypes {
		if t == contentType {
			allowed = true
			break
		}
	}
	if !allowed {
		respondWithError(w, http.StatusBadRequest, "Unsupported file type")
		return
	}

	os.MkdirAll(setting.Directory, os.ModePerm)

	newName := fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(handler.Filename))
	savePath := filepath.Join(setting.Directory, newName)
	out, _ := os.Create(savePath)
	defer out.Close()
	io.Copy(out, file)

	// DB 저장
	res, _ := db.Exec("INSERT INTO files (original_name, saved_name, use_type, content_type, size, created_at) VALUES (?, ?, ?, ?, ?, NOW())",
		handler.Filename, newName, useType, contentType, handler.Size)
	fileID, _ := res.LastInsertId()

	// 썸네일 생성
	if strings.HasPrefix(contentType, "image/") {
		createThumbnail(savePath, newName)
	}

	// 히스토리 기록
	logFileHistory("system", "upload", fileID)

	respondWithJSON(w, map[string]interface{}{"id": fileID})
}

// 썸네일 생성
func createThumbnail(filePath, fileName string) {
	infile, _ := os.Open(filePath)
	defer infile.Close()

	var img image.Image
	var err error

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == ".jpg" || ext == ".jpeg" {
		img, err = jpeg.Decode(infile)
	} else if ext == ".png" {
		img, err = png.Decode(infile)
	} else {
		return
	}

	if err != nil {
		return
	}

	m := resize.Thumbnail(200, 200, img, resize.Lanczos3)
	outPath := filepath.Join("uploads", "thumbnails", fileName)
	outfile, _ := os.Create(outPath)
	defer outfile.Close()
	jpeg.Encode(outfile, m, nil)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("id")
	if fileID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing file ID")
		return
	}

	var originalName, savedName, useType, contentType string
	err := db.QueryRow("SELECT original_name, saved_name, use_type, content_type FROM files WHERE id = ?", fileID).
		Scan(&originalName, &savedName, &useType, &contentType)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	setting := uploadSettings[useType]
	filePath := filepath.Join(setting.Directory, savedName)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", originalName))
	http.ServeFile(w, r, filePath)

	// 히스토리 기록
	logFileHistory("system", "download", toInt64(fileID))
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("id")
	if fileID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing file ID")
		return
	}

	var savedName, useType string
	err := db.QueryRow("SELECT saved_name, use_type FROM files WHERE id = ?", fileID).
		Scan(&savedName, &useType)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "File not found")
		return
	}

	setting := uploadSettings[useType]
	filePath := filepath.Join(setting.Directory, savedName)

	os.Remove(filePath)
	db.Exec("DELETE FROM files WHERE id = ?", fileID)

	// 히스토리 기록
	logFileHistory("system", "delete", toInt64(fileID))

	respondWithJSON(w, map[string]string{"message": "File deleted"})
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := toInt(pageStr)
	if page == 0 {
		page = 1
	}
	pageSize := toInt(pageSizeStr)
	if pageSize == 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	rows, err := db.Query("SELECT id, original_name, use_type, size, created_at FROM files ORDER BY id DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type File struct {
		ID           int64  `json:"id"`
		OriginalName string `json:"original_name"`
		UseType      string `json:"use_type"`
		Size         int64  `json:"size"`
		CreatedAt    string `json:"created_at"`
	}

	var files []File
	for rows.Next() {
		var f File
		rows.Scan(&f.ID, &f.OriginalName, &f.UseType, &f.Size, &f.CreatedAt)
		files = append(files, f)
	}

	respondWithJSON(w, map[string]interface{}{
		"files":    files,
		"page":     page,
		"pageSize": pageSize,
	})
}

func toInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func toInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}

func main() {
	var err error
	db, err = sql.Open("mysql", "root:sjrnfl0944@tcp(localhost:3306)/file_server")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	os.MkdirAll("uploads/profile", os.ModePerm)
	os.MkdirAll("uploads/document", os.ModePerm)
	os.MkdirAll("uploads/thumbnails", os.ModePerm)

	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/refresh", refreshHandler)
	http.HandleFunc("/upload", authMiddleware(uploadHandler, false))
	http.HandleFunc("/download", authMiddleware(downloadHandler, false))
	http.HandleFunc("/delete", authMiddleware(deleteHandler, true)) // 삭제는 admin만
	http.HandleFunc("/files", authMiddleware(listFilesHandler, false))

	// 정적 파일 서빙
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", nil)
}
