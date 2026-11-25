module cloudbin-api-gateway

go 1.27.0

require (
	cloudbin-shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
)

replace cloudbin-shared => ../shared
