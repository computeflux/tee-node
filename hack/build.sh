# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

tag=`date "+%Y-%m-%d-%H_%M"`

# build binary
ego-go build -o ./hack/build/dsecret ./main.go

# sign binary
cd ./hack/build/
ego sign dsecret

cd $DIR/../

# 构建镜像
docker build -t wetee/dsecret:$tag .
docker push wetee/dsecret:$tag

echo "wetee/dsecret:$tag" > ./hack/.version