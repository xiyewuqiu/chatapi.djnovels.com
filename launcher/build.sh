CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o ./build/im.djnovels.com main.go


cp -rf scripts/* ./build/
