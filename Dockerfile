FROM golang:1.18

COPY . /app

WORKDIR /app

RUN go mod tidy

ENTRYPOINT ["go", "run", "main.go"]