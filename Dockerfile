FROM golang:1.23
RUN mkdir /app
WORKDIR /app
COPY . /app/
RUN go get github.com/valkey-io/valkey-go
CMD ["go","run","main.go"]
