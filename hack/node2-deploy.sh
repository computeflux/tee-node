# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

img=$(cat ./hack/.version)

export DSECRET_IMAGE=$img
export DSECRET_DIR=/srv/token_node2/token_chain_data
export NAME=tee-provider-2

export SIDE_CHAIN_PORT=30136
export SIDE_CHAIN_RPC_PORT=30137
export GQL_PORT=30138
export CHAIN_ENV=test

export JWT_SECRET="${JWT_SECRET:-dev-jwt-secret-change-in-production}"
export SEED_SECRET="${SEED_SECRET:-dev-seed-secret-change-in-production}"
export MOCK_EMAIL="${MOCK_EMAIL:-true}"
export CORS_ORIGINS="${CORS_ORIGINS:-http://localhost:5173,http://xiaobai.asyou.me:5174}"

# ---- Google OAuth ----
export GOOGLE_CLIENT_ID="${GOOGLE_CLIENT_ID:-560483017331-cjvd4d7ed90ddibspo72fmdmc87nb91v.apps.googleusercontent.com}"
export GOOGLE_CLIENT_SECRET="${GOOGLE_CLIENT_SECRET:-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}"
export GOOGLE_REDIRECT_URL="${GOOGLE_REDIRECT_URL:-http://xiaobai.asyou.me:5174/oauth/callback.html}"

# ---- GitHub OAuth (optional) ----
export GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID:-}"
export GITHUB_CLIENT_SECRET="${GITHUB_CLIENT_SECRET:-}"
export GITHUB_REDIRECT_URL="${GITHUB_REDIRECT_URL:-}"

# ---- X/Twitter OAuth (optional) ----
export TWITTER_CLIENT_ID="${TWITTER_CLIENT_ID:-}"
export TWITTER_CLIENT_SECRET="${TWITTER_CLIENT_SECRET:-}"
export TWITTER_REDIRECT_URL="${TWITTER_REDIRECT_URL:-}"

# ---- SMTP (email OTP) ----
export SMTP_HOST="${SMTP_HOST:-}"
export SMTP_PORT="${SMTP_PORT:-587}"
export SMTP_USER="${SMTP_USER:-}"
export SMTP_PASSWORD="${SMTP_PASSWORD:-}"
export SMTP_FROM="${SMTP_FROM:-}"
export SMTP_FROM_NAME="${SMTP_FROM_NAME:-TEE Auth}"

echo '' > ./hack/k8s.yaml
envsubst < ./hack/k8s-temp.yaml > ./hack/k8s.yaml

# 部署镜像
kubectl delete deployment tee-provider-2 -n worker-addon
kubectl delete service tee-provider-2-service -n worker-addon
kubectl create -f ./hack/k8s.yaml
