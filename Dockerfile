FROM golang:1.24.1-alpine3.21 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ./main.exe ./main.go

FROM build-stage AS dev-stage

WORKDIR /app

COPY --from=build-stage /app/main.exe /main

ENTRYPOINT ["./main.exe"]