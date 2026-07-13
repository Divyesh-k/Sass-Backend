module github.com/divyeshkakadiya/saas-backend

go 1.22

replace golang.org/x/sys => github.com/golang/sys v0.12.0

replace golang.org/x/text => github.com/golang/text v0.14.0

replace golang.org/x/crypto => github.com/golang/crypto v0.21.0

replace golang.org/x/net => github.com/golang/net v0.21.0

require (
	github.com/alicebob/miniredis/v2 v2.32.1
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/redis/go-redis/v9 v9.5.1
	golang.org/x/crypto v0.0.0-00010101000000-000000000000
)

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)
