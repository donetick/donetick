FROM golang:1.22

WORKDIR /usr/src/app


COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w " -buildvcs=false  -o /donetick-core

EXPOSE 8080
CMD ["donetick-core"]