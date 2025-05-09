FROM golang:1.24-alpine

WORKDIR /app
COPY go.mod ./
COPY go.sum ./

RUN go mod download
COPY . ./

# You need CGO_ENABLED=0 to make it so the binary isn't dynamically linked
# for more information: https://stackoverflow.com/a/55106860/57626
RUN CGO_ENABLED=0 GOOS=linux go build -o /unifi-dns-scraper

FROM scratch

COPY --from=0 /unifi-dns-scraper /unifi-dns-scraper

CMD ["/unifi-dns-scraper", "-config", "/config.toml"]
