$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent $PSScriptRoot
Set-Location $repo

$gopath = (& go env GOPATH)
$env:Path = "$env:Path;$gopath\bin"

New-Item -ItemType Directory -Force -Path "src/rpc/pb" | Out-Null

protoc `
  --proto_path=proto `
  --go_out=paths=source_relative:. `
  --go-grpc_out=paths=source_relative:. `
  proto/im.proto

Move-Item -Force -Path "im.pb.go" -Destination "src\rpc\pb\im.pb.go"
Move-Item -Force -Path "im_grpc.pb.go" -Destination "src\rpc\pb\im_grpc.pb.go"

Write-Host "proto generated: proto/im.proto -> src/rpc/pb/*.pb.go"
