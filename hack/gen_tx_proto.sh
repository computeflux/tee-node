# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../pkg/model


protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-vtproto_out=. --go-vtproto_opt=paths=source_relative \
  --go-vtproto_opt=features=marshal+unmarshal+size \
  tx.proto

rm -f ../../../libos-entry/model/tx.pb.go
cp tx.pb.go ../../../libos-entry/model/
cp -r protoio ../../../libos-entry/model/
