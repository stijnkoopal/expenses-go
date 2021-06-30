# build stage
FROM golang:alpine AS build-env
ADD src/. /src
RUN cd /src && go build -o expenses

# execution stage
FROM alpine
WORKDIR /app
COPY --from=build-env /src/expenses /app/
ENTRYPOINT ./expenses