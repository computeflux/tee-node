package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/computeflux/tee-provider/pkg/api/auth"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/vedhavyas/go-subkey/v2"
	"github.com/vedhavyas/go-subkey/v2/sr25519"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

var (
	loginStatCtxKey = &contextKey{"loginStat"}
)

type contextKey struct {
	name string
}

// JWTValidator parses a JWT token string and returns claims.
type JWTValidator func(tokenStr string) (*auth.JWTClaims, error)

// AuthCheck checks the user's role and timestamp
func AuthCheck(ctx context.Context, obj interface{}, next graphql.Resolver) (res interface{}, err error) {
	user := ctx.Value(loginStatCtxKey).(*model.PublicUser)
	if user == nil {
		err = gqlerror.Errorf("Please log in first.")
		return
	}

	if user.Timestamp+3 < time.Now().Unix() {
		fmt.Println("Login expired, please log in again.")
		err = gqlerror.Errorf("Login expired, please log in again.")
		return
	}
	return next(ctx)
}

// AuthMiddleware creates a middleware that decodes the Authorization header
// and packs the user into context. Supports both the legacy Polkadot (||)
// format and JWT Bearer tokens.
func AuthMiddleware(validateJWT JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := InitUser(r, validateJWT)

			// put it in context
			ctx := context.WithValue(r.Context(), loginStatCtxKey, user)

			// and call the next with our new context
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// InitUser decodes the Authorization header. Supports:
//   - Legacy Polkadot format: hex(JSON{address,timestamp})||hex(signature)
//   - JWT Bearer format: Bearer <jwt>
func InitUser(r *http.Request, validateJWT JWTValidator) *model.PublicUser {
	header := r.Header
	token, ok := header["Authorization"]
	if !ok || len(token) == 0 {
		return nil
	}

	raw := token[0]

	// 1. Try JWT Bearer token first.
	if strings.HasPrefix(raw, "Bearer ") {
		return decodeJWT(strings.TrimPrefix(raw, "Bearer "), validateJWT)
	}

	// 2. Try legacy Polkadot || format.
	if strings.Contains(raw, "||") {
		return decodeLegacyToken(raw)
	}

	// 3. Fallback: could be a bare JWT.
	if validateJWT != nil {
		return decodeJWT(raw, validateJWT)
	}

	return nil
}

// decodeJWT parses a JWT token and returns a PublicUser.
func decodeJWT(tokenStr string, validateJWT JWTValidator) *model.PublicUser {
	if validateJWT == nil {
		return nil
	}
	claims, err := validateJWT(tokenStr)
	if err != nil {
		return nil
	}
	return &model.PublicUser{
		Address:   claims.Address,
		Timestamp: time.Now().Unix(),
	}
}

// decodeLegacyToken decodes the legacy Polkadot ||-separated token format.
func decodeLegacyToken(tokenStr string) *model.PublicUser {
	token := strings.Split(tokenStr, "||")
	if len(token) != 2 {
		return nil
	}

	bt, terr := subkey.DecodeHex(token[0])
	if !terr {
		fmt.Println(terr)
		return nil
	}

	user := &model.PublicUser{}
	err := json.Unmarshal(bt, user)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// 解析地址
	_, pubkeyBytes, err := subkey.SS58Decode(user.Address)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// 解析公钥
	pubkey, err := sr25519.Scheme{}.FromPublicKey(pubkeyBytes)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// 解析签名
	sig, chainerr := subkey.DecodeHex(token[1])
	if !chainerr {
		fmt.Println(err)
		return nil
	}

	// 构造签名内容
	uinput := model.PublicUser{
		Address:   user.Address,
		Timestamp: user.Timestamp,
	}
	inputbt, _ := json.Marshal(uinput)

	// 验证签名
	ok := pubkey.Verify([]byte("<Bytes>"+string(inputbt)+"</Bytes>"), sig)
	if !ok {
		return nil
	}

	return user
}
