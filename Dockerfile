FROM golang AS build
LABEL authors="Adam Ondrejcak <adam@ondrejcak.sk>"

WORKDIR /app
COPY go.* .
RUN go mod download

COPY . .
RUN go build -o dist/app

FROM golang AS final

WORKDIR /app
COPY --from=build /app/dist/app .
COPY ./start.sh .

ENTRYPOINT ["/app/start.sh"]
