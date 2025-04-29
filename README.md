# goFileServer
gpt가 생성한 파일서버 

# File Server (Go)

## 기능
- 파일 업로드/다운로드/삭제
- JWT 인증 (Secure Cookie)
- Refresh Token Rotation
- Role 기반 접근 제어 (admin/user)
- 파일 업로드시 썸네일 자동 생성
- 파일 업로드/다운로드/삭제 히스토리 기록

## 설치 방법
```bash
go mod tidy
mysql -u root -p yourdb < db/schema.sql
go run main.go